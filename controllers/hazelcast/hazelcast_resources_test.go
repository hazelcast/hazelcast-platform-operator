package hazelcast

import (
	"context"
	"testing"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/config"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

func Test_hazelcastConfigMapMultipleCRs(t *testing.T) {
	meta := metav1.ObjectMeta{
		Name:      "hazelcast",
		Namespace: "default",
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: meta,
	}
	h := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: meta,
	}

	hzConfig := &config.HazelcastWrapper{}
	err := yaml.Unmarshal([]byte(cm.Data["hazelcast.yaml"]), hzConfig)
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
			data, err := hazelcastConfigMapData(context.Background(), c, h)
			if err != nil {
				t.Errorf("Error retreiving ConfigMap data")
			}
			actualConfig := &config.HazelcastWrapper{}
			err = yaml.Unmarshal([]byte(data["hazelcast.yaml"]), actualConfig)
			if err != nil {
				t.Errorf("Error unmarshaling actial Hazelcast config YAML")
			}
			Expect(test.listKeys(actualConfig.Hazelcast)).Should(HaveLen(len(crNames)))
			for _, name := range crNames {
				Expect(test.listKeys(actualConfig.Hazelcast)).Should(ContainElement(name))
			}
		})
	}
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
