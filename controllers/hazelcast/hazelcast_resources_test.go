package hazelcast

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/config"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

func Test_hazelcastConfigMultipleCRs(t *testing.T) {
	meta := metav1.ObjectMeta{
		Name:      "hazelcast",
		Namespace: "default",
	}
	cm := &corev1.Secret{
		ObjectMeta: meta,
	}
	h := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: meta,
	}

	hzConfig := &config.HazelcastWrapper{}
	err := yaml.Unmarshal(cm.Data["hazelcast.yaml"], hzConfig)
	if err != nil {
		t.Errorf("Error unmarshalling Hazelcast config")
	}
	structureSpec := hazelcastv1alpha1.DataStructureSpec{
		HazelcastResourceName: meta.Name,
		BackupCount:           pointer.Int32(1),
		AsyncBackupCount:      0,
	}
	structureStatus := hazelcastv1alpha1.DataStructureStatus{State: hazelcastv1alpha1.DataStructureSuccess}

	tests := []struct {
		name     string
		listKeys listKeys
		c        client.Object
	}{
		{
			name: "Cache CRs",
			listKeys: func(h config.Hazelcast) []string {
				return getKeys(h.Cache)
			},
			c: &hazelcastv1alpha1.Cache{
				TypeMeta: metav1.TypeMeta{
					Kind: "Cache",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec:   hazelcastv1alpha1.CacheSpec{DataStructureSpec: structureSpec},
				Status: hazelcastv1alpha1.CacheStatus{DataStructureStatus: structureStatus},
			},
		},
		{
			name: "Topic CRs",
			listKeys: func(h config.Hazelcast) []string {
				return getKeys(h.Topic)
			},
			c: &hazelcastv1alpha1.Topic{
				TypeMeta: metav1.TypeMeta{
					Kind: "Topic",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec:   hazelcastv1alpha1.TopicSpec{HazelcastResourceName: meta.Name},
				Status: hazelcastv1alpha1.TopicStatus{DataStructureStatus: structureStatus},
			},
		},
		{
			name: "MultiMap CRs",
			listKeys: func(h config.Hazelcast) []string {
				return getKeys(h.MultiMap)
			},
			c: &hazelcastv1alpha1.MultiMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "MultiMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec:   hazelcastv1alpha1.MultiMapSpec{DataStructureSpec: structureSpec},
				Status: hazelcastv1alpha1.MultiMapStatus{DataStructureStatus: structureStatus},
			},
		},
		{
			name: "ReplicatedMap CRs",
			listKeys: func(h config.Hazelcast) []string {
				return getKeys(h.ReplicatedMap)
			},
			c: &hazelcastv1alpha1.ReplicatedMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "ReplicatedMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec:   hazelcastv1alpha1.ReplicatedMapSpec{HazelcastResourceName: meta.Name, AsyncFillup: pointer.Bool(true)},
				Status: hazelcastv1alpha1.ReplicatedMapStatus{DataStructureStatus: structureStatus},
			},
		},
		{
			name: "Queue CRs",
			listKeys: func(h config.Hazelcast) []string {
				return getKeys(h.Queue)
			},
			c: &hazelcastv1alpha1.Queue{
				TypeMeta: metav1.TypeMeta{
					Kind: "Queue",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.QueueSpec{
					EmptyQueueTtlSeconds: pointer.Int32(10),
					MaxSize:              0,
					DataStructureSpec:    structureSpec,
				},
				Status: hazelcastv1alpha1.QueueStatus{DataStructureStatus: structureStatus},
			},
		},
		{
			name: "Map CRs",
			listKeys: func(h config.Hazelcast) []string {
				return getKeys(h.Map)
			},
			c: &hazelcastv1alpha1.Map{
				TypeMeta: metav1.TypeMeta{
					Kind: "Map",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cr",
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.MapSpec{
					DataStructureSpec: structureSpec,
					TimeToLiveSeconds: 10,
					Eviction: hazelcastv1alpha1.EvictionConfig{
						MaxSize: 0,
					},
				},
				Status: hazelcastv1alpha1.MapStatus{State: hazelcastv1alpha1.MapSuccess},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			RegisterFailHandler(fail(t))
			crNames := []string{"cr-name-1", "another-created-cr", "custom-resource"}
			objects := make([]client.Object, len(crNames))
			for i := range crNames {
				test.c.SetName(crNames[i])
				objects[i] = test.c.DeepCopyObject().(client.Object)
			}
			objects = append(objects, cm, h)
			c := fakeK8sClient(objects...)
			data, err := hazelcastConfig(context.Background(), c, h, logr.Discard())
			if err != nil {
				t.Errorf("Error retreiving Secret data")
			}
			actualConfig := &config.HazelcastWrapper{}
			err = yaml.Unmarshal(data, actualConfig)
			if err != nil {
				t.Errorf("Error unmarshaling actial Hazelcast config YAML")
			}
			listKeys := test.listKeys(actualConfig.Hazelcast)
			Expect(listKeys).Should(HaveLen(len(crNames)))
			for _, name := range crNames {
				Expect(listKeys).Should(ContainElement(name))
			}
		})
	}
}

func Test_hazelcastConfigMultipleWanCRs(t *testing.T) {
	hz := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hazelcast",
			Namespace: "default",
		},
	}
	wrs := &hazelcastv1alpha1.WanReplicationList{}
	wrs.Items = []hazelcastv1alpha1.WanReplication{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wan-1",
				Namespace: "default",
			},
			Spec: hazelcastv1alpha1.WanReplicationSpec{
				WanPublisherConfig: hazelcastv1alpha1.WanPublisherConfig{
					Resources: []hazelcastv1alpha1.ResourceSpec{
						{
							Name: hz.Name,
							Kind: hazelcastv1alpha1.ResourceKindHZ,
						},
					},
					TargetClusterName: "dev",
					Endpoints:         "10.0.0.1:5701",
				},
			},
			Status: hazelcastv1alpha1.WanReplicationStatus{
				WanReplicationMapsStatus: map[string]hazelcastv1alpha1.WanReplicationMapStatus{
					hz.Name + "__map": {
						PublisherId: "map-wan-1",
						Status:      hazelcastv1alpha1.WanStatusSuccess,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wan-2",
				Namespace: "default",
			},
			Spec: hazelcastv1alpha1.WanReplicationSpec{
				WanPublisherConfig: hazelcastv1alpha1.WanPublisherConfig{
					Resources: []hazelcastv1alpha1.ResourceSpec{
						{
							Name: hz.Name,
							Kind: hazelcastv1alpha1.ResourceKindHZ,
						},
					},
					TargetClusterName: "dev",
					Endpoints:         "10.0.0.2:5701",
				},
			},
			Status: hazelcastv1alpha1.WanReplicationStatus{
				WanReplicationMapsStatus: map[string]hazelcastv1alpha1.WanReplicationMapStatus{
					hz.Name + "__map": {
						PublisherId: "map-wan-2",
						Status:      hazelcastv1alpha1.WanStatusSuccess,
					},
				},
			},
		},
	}
	objects := []client.Object{
		hz,
		&hazelcastv1alpha1.Map{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "map",
				Namespace: "default",
			},
			Spec: hazelcastv1alpha1.MapSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: hz.Name,
				},
			},
		},
	}
	c := &mockK8sClient{Client: fakeK8sClient(objects...)}
	c.list = wrs

	bytes, err := hazelcastConfig(context.TODO(), c, hz, logr.Discard())
	if err != nil {
		t.Errorf("unable to build Config, %e", err)
	}
	hzConfig := &config.HazelcastWrapper{}
	err = yaml.Unmarshal(bytes, hzConfig)
	if err != nil {
		t.Error(err)
	}
	wrConf, ok := hzConfig.Hazelcast.WanReplication["map-default"]
	if !ok {
		t.Errorf("wan config for map-default not found")
	}
	if _, ok := wrConf.BatchPublisher["map-wan-1"]; !ok {
		t.Errorf("butch publisher map-wan-1 not found")
	}
	if _, ok := wrConf.BatchPublisher["map-wan-2"]; !ok {
		t.Errorf("butch publisher map-wan-2 not found")
	}
}

func Test_hazelcastConfig(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		hzSpec         hazelcastv1alpha1.HazelcastSpec
		expectedResult interface{}
		actualResult   func(h config.Hazelcast) interface{}
	}{
		{
			name:           "Empty Compact Serialization",
			hzSpec:         hazelcastv1alpha1.HazelcastSpec{},
			expectedResult: (*config.CompactSerialization)(nil),
			actualResult: func(h config.Hazelcast) interface{} {
				return h.Serialization.CompactSerialization
			},
		},
		{
			name: "Compact Serialization Class",
			hzSpec: hazelcastv1alpha1.HazelcastSpec{
				Serialization: &hazelcastv1alpha1.SerializationConfig{
					CompactSerialization: &hazelcastv1alpha1.CompactSerializationConfig{
						Classes: []string{"test1", "test2"},
					},
				},
			},
			expectedResult: []string{
				"class: test1",
				"class: test2",
			},
			actualResult: func(h config.Hazelcast) interface{} {
				return h.Serialization.CompactSerialization.Classes
			},
		},
		{
			name: "Compact Serialization Serializer",
			hzSpec: hazelcastv1alpha1.HazelcastSpec{
				Serialization: &hazelcastv1alpha1.SerializationConfig{
					CompactSerialization: &hazelcastv1alpha1.CompactSerializationConfig{
						Serializers: []string{"test1", "test2"},
					},
				},
			},
			expectedResult: []string{
				"serializer: test1",
				"serializer: test2",
			},
			actualResult: func(h config.Hazelcast) interface{} {
				return h.Serialization.CompactSerialization.Serializers
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			RegisterFailHandler(fail(t))
			h := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hazelcast",
					Namespace: "default",
				},
				Spec: test.hzSpec,
			}

			c := fakeK8sClient(h)
			data, err := hazelcastConfig(context.Background(), c, h, logr.Discard())
			if err != nil {
				t.Errorf("Error retreiving Secret data")
			}
			actualConfig := &config.HazelcastWrapper{}
			err = yaml.Unmarshal(data, actualConfig)
			if err != nil {
				t.Errorf("Error unmarshaling actial Hazelcast config YAML")
			}

			Expect(test.actualResult(actualConfig.Hazelcast)).Should(Equal(test.expectedResult))
		})
	}
}

// Client used to mock List() method for the WanReplicationList CR
// Needed since the List() method uses the indexed filed "hazelcastResourceName" not available in the fakeClient.
type mockK8sClient struct {
	client.Client
	list *hazelcastv1alpha1.WanReplicationList
}

func (c *mockK8sClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if list == nil || reflect.TypeOf(list) != reflect.TypeOf(c.list) {
		return c.Client.List(ctx, list, opts...)
	}
	list.(*hazelcastv1alpha1.WanReplicationList).Items = c.list.Items
	return nil
}

type listKeys func(h config.Hazelcast) []string

func getKeys[C config.Cache | config.ReplicatedMap |
	config.MultiMap | config.Topic | config.Queue | config.Map](m map[string]C) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestDeepMerge(t *testing.T) {
	tests := []struct {
		src  map[string]any
		dst  map[string]any
		want map[string]any
	}{
		// empty
		{
			src:  make(map[string]any),
			dst:  make(map[string]any),
			want: make(map[string]any),
		},
		// union
		{
			src: map[string]any{
				"foo": "foo",
			},
			dst: map[string]any{
				"bar": "bar",
			},
			want: map[string]any{
				"foo": "foo",
				"bar": "bar",
			},
		},
		// map merge
		{
			src: map[string]any{
				"foo": map[string]any{
					"bar": "bar",
				},
			},
			dst: map[string]any{
				"foo": map[string]any{
					"baz": "baz",
				},
			},
			want: map[string]any{
				"foo": map[string]any{
					"bar": "bar",
					"baz": "baz",
				},
			},
		},
		// value overwrite
		{
			src: map[string]any{
				"foo": "bar",
			},
			dst: map[string]any{
				"foo": "baz",
			},
			want: map[string]any{
				"foo": "bar",
			},
		},
		// example
		{
			src: map[string]any{
				"hazelcast": map[string]any{
					"persistence": map[string]any{
						"enabled": true,
					},
					"map": map[string]any{
						"test-map": map[string]any{
							"data-persistence": map[string]any{
								"enabled": true,
								"fsync":   true,
							},
						},
					},
				},
			},
			dst: map[string]any{
				"hazelcast": map[string]any{
					"persistence": map[string]any{
						"enabled":    true,
						"base-dir":   "/mnt/persistence",
						"backup-dir": "/mnt/hot-backup",
					},
					"map": map[string]any{
						"backup-count": 1,
					},
				},
			},
			want: map[string]any{
				"hazelcast": map[string]any{
					"persistence": map[string]any{
						"enabled":    true,
						"base-dir":   "/mnt/persistence",
						"backup-dir": "/mnt/hot-backup",
					},
					"map": map[string]any{
						"backup-count": 1,
						"test-map": map[string]any{
							"data-persistence": map[string]any{
								"enabled": true,
								"fsync":   true,
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		deepMerge(tc.dst, tc.src)
		if diff := cmp.Diff(tc.want, tc.dst); diff != "" {
			t.Errorf("deepMerge(%v, %v) mismatch (-want, +got):\n%s", tc.dst, tc.src, diff)
		}
	}
}
