package hazelcast

import (
	"context"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
)

func Test_clientShutdownWhenConnectionNotEstablished(t *testing.T) {
	h := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hazelcast",
			Namespace: "default",
		},
	}
	r := reconcilerWithCR(h)
	hzclient.Clients.Store(types.NamespacedName{Name: h.Name, Namespace: h.Namespace}, &hzclient.Client{})

	err := r.executeFinalizer(context.Background(), h, ctrl.Log)
	if err != nil {
		t.Errorf("Error while executing finilazer: %v.", err)
	}
}

func reconcilerWithCR(h *hazelcastv1alpha1.Hazelcast) HazelcastReconciler {
	return HazelcastReconciler{
		Client: fakeClient(h),
	}
}

func Test_hazelcastConfigMapData(t *testing.T) {
	RegisterFailHandler(fail(t))
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
	cacheSpec := hazelcastv1alpha1.CacheSpec{
		DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
			HazelcastResourceName: meta.Name,
			BackupCount:           pointer.Int32Ptr(1),
			AsyncBackupCount:      pointer.Int32Ptr(0),
		},
	}
	cacheStatus := hazelcastv1alpha1.CacheStatus{
		DataStructureStatus: hazelcastv1alpha1.DataStructureStatus{
			State: hazelcastv1alpha1.DataStructureSuccess,
		},
	}
	typeMeta := metav1.TypeMeta{
		Kind: "Cache",
	}
	cache1 := &hazelcastv1alpha1.Cache{
		TypeMeta: typeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cache-1",
			Namespace: "default",
		},
		Spec:   cacheSpec,
		Status: cacheStatus,
	}
	cache2 := &hazelcastv1alpha1.Cache{
		TypeMeta: typeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cache-2",
			Namespace: "default",
		},
		Spec:   cacheSpec,
		Status: cacheStatus,
	}
	client := fakeClient(cm, h, cache1, cache2)

	data, err := hazelcastConfigMapData(context.Background(), client, h)
	if err != nil {
		t.Errorf("Error retreiving ConfigMap data")
	}
	actualConfig := &config.HazelcastWrapper{}
	err = yaml.Unmarshal([]byte(data["hazelcast.yaml"]), actualConfig)
	if err != nil {
		t.Errorf("Error unmarshaling actial Hazelcast config YAML")
	}
	Expect(actualConfig.Hazelcast.Cache).Should(And(HaveKey("cache-1"), HaveKey("cache-2")))
}
