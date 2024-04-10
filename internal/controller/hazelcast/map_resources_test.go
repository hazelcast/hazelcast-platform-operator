package hazelcast

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func Test_mapTieredStoreConfig(t *testing.T) {
	nn, h, m := defaultCRsMap()
	h.Spec.NativeMemory = &hazelcastv1alpha1.NativeMemoryConfiguration{
		AllocatorType: hazelcastv1alpha1.NativeMemoryPooled,
		Size:          []resource.Quantity{resource.MustParse("512M")}[0],
	}
	h.Spec.LocalDevices = []hazelcastv1alpha1.LocalDeviceConfig{
		{
			Name: "test-device",
			PVC: &hazelcastv1alpha1.PvcConfiguration{
				AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				RequestStorage: &[]resource.Quantity{resource.MustParse("256M")}[0],
			},
		},
	}

	tests := []struct {
		name       string
		mapSpec    hazelcastv1alpha1.MapSpec
		errMessage string
	}{
		{
			name: "Wrong InMemoryFormat",
			mapSpec: hazelcastv1alpha1.MapSpec{
				TieredStore: &hazelcastv1alpha1.TieredStore{
					DiskDeviceName: "test-device",
					MemoryCapacity: &[]resource.Quantity{resource.MustParse("128M")}[0],
				},
			},
			errMessage: "In-memory format of the map must be NATIVE to enable the Tiered Storage",
		},
		{
			name: "Persistence Enabled",
			mapSpec: hazelcastv1alpha1.MapSpec{
				PersistenceEnabled: true,
				InMemoryFormat:     hazelcastv1alpha1.InMemoryFormatNative,
				TieredStore: &hazelcastv1alpha1.TieredStore{
					DiskDeviceName: "test-device",
					MemoryCapacity: &[]resource.Quantity{resource.MustParse("128M")}[0],
				},
			},
			errMessage: "Tiered store and data persistence are mutually exclusive features. Persistence must be disabled to enable the Tiered Storage",
		},
		{
			name: "Index Configured",
			mapSpec: hazelcastv1alpha1.MapSpec{
				Indexes: []hazelcastv1alpha1.IndexConfig{
					{
						Type: hazelcastv1alpha1.IndexTypeHash,
					},
				},
				InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
				TieredStore: &hazelcastv1alpha1.TieredStore{
					DiskDeviceName: "test-device",
					MemoryCapacity: &[]resource.Quantity{resource.MustParse("128M")}[0],
				},
			},
			errMessage: "Indexes is not supported for Tiered-Store map",
		},
		{
			name: "Eviction Configured",
			mapSpec: hazelcastv1alpha1.MapSpec{
				Eviction: hazelcastv1alpha1.EvictionConfig{
					EvictionPolicy: "LRU",
				},
				InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
				TieredStore: &hazelcastv1alpha1.TieredStore{
					DiskDeviceName: "test-device",
					MemoryCapacity: &[]resource.Quantity{resource.MustParse("128M")}[0],
				},
			},
			errMessage: "Eviction is not supported for Tiered-Store map",
		},
		{
			name: "TTL Configured",
			mapSpec: hazelcastv1alpha1.MapSpec{
				TimeToLiveSeconds: int32(100),
				InMemoryFormat:    hazelcastv1alpha1.InMemoryFormatNative,
				TieredStore: &hazelcastv1alpha1.TieredStore{
					DiskDeviceName: "test-device",
					MemoryCapacity: &[]resource.Quantity{resource.MustParse("128M")}[0],
				},
			},
			errMessage: "TTL expiry is not supported for Tiered-Store map",
		},
		{
			name: "MaxIdle Configured",
			mapSpec: hazelcastv1alpha1.MapSpec{
				MaxIdleSeconds: int32(100),
				InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
				TieredStore: &hazelcastv1alpha1.TieredStore{
					DiskDeviceName: "test-device",
					MemoryCapacity: &[]resource.Quantity{resource.MustParse("128M")}[0],
				},
			},
			errMessage: "MaxIdle expiry is not supported for Tiered-Store map",
		},
		{
			name: "LocalDevice does not exist",
			mapSpec: hazelcastv1alpha1.MapSpec{
				InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
				TieredStore: &hazelcastv1alpha1.TieredStore{
					DiskDeviceName: "missing-device",
					MemoryCapacity: &[]resource.Quantity{resource.MustParse("128M")}[0],
				},
			},
			errMessage: "device with the name missing-device does not exist",
		},
		{
			name: "Memory tier is bigger than Disk tier",
			mapSpec: hazelcastv1alpha1.MapSpec{
				InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
				TieredStore: &hazelcastv1alpha1.TieredStore{
					DiskDeviceName: "test-device",
					MemoryCapacity: &[]resource.Quantity{resource.MustParse("512M")}[0],
				},
			},
			errMessage: "Tiered Storage in-memory tier must be smaller than the disk tier",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			RegisterFailHandler(fail(t))
			hs, _ := json.Marshal(h.Spec)
			h.ObjectMeta.Annotations = map[string]string{
				n.LastSuccessfulSpecAnnotation: string(hs),
			}
			m.Spec = test.mapSpec
			m.Spec.HazelcastResourceName = h.Name
			r := mapReconcilerWithCRs(&fakeHzClientRegistry{}, h, m)
			_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nn})
			if err == nil {
				t.Errorf("Error expecting Reconcile to return error")
			}

			Eventually(func() hazelcastv1alpha1.MapConfigState {
				_ = r.Client.Get(context.TODO(), nn, m)
				return m.Status.State
			}, 2*time.Second, 100*time.Millisecond).Should(Equal(hazelcastv1alpha1.MapFailed))
			Expect(m.Status.Message).Should(ContainSubstring(test.errMessage))
		})
	}

}

func defaultCRsMap() (types.NamespacedName, *hazelcastv1alpha1.Hazelcast, *hazelcastv1alpha1.Map) {
	nn := types.NamespacedName{
		Name:      "hazelcast",
		Namespace: "default",
	}
	h := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: hazelcastv1alpha1.HazelcastSpec{},
		Status: hazelcastv1alpha1.HazelcastStatus{
			Phase: hazelcastv1alpha1.Running,
		},
	}
	hs, _ := json.Marshal(h.Spec)
	h.ObjectMeta.Annotations = map[string]string{
		n.LastSuccessfulSpecAnnotation: string(hs),
	}
	m := &hazelcastv1alpha1.Map{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: hazelcastv1alpha1.MapSpec{
			DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
				HazelcastResourceName: nn.Name,
			},
		},
	}
	return nn, h, m
}

func mapReconcilerWithCRs(clientReg hzclient.ClientRegistry, initObjs ...client.Object) *MapReconciler {
	return NewMapReconciler(
		fakeK8sClient(initObjs...),
		ctrl.Log.WithName("test").WithName("Hazelcast"),
		nil,
		clientReg,
	)
}
