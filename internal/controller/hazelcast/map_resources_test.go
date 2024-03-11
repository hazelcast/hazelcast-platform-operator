package hazelcast

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/gomega"
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
	}

	tests := []struct {
		name        string
		localDevice hazelcastv1alpha1.LocalDeviceConfig
		mapSpec     hazelcastv1alpha1.MapSpec
		errMessage  string
	}{
		{
			name:        "Wrong InMemoryFormat",
			localDevice: hazelcastv1alpha1.LocalDeviceConfig{Name: "test-device"},
			mapSpec: hazelcastv1alpha1.MapSpec{
				TieredStore: &hazelcastv1alpha1.TieredStore{
					DiskDeviceName: "test-device",
				},
			},
			errMessage: "In-memory format of the map must be NATIVE to enable the Tiered Storage",
		},
		{
			name:        "Index Configured",
			localDevice: hazelcastv1alpha1.LocalDeviceConfig{Name: "test-device"},
			mapSpec: hazelcastv1alpha1.MapSpec{
				Indexes: []hazelcastv1alpha1.IndexConfig{
					{
						Type: hazelcastv1alpha1.IndexTypeHash,
					},
				},
				InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
				TieredStore: &hazelcastv1alpha1.TieredStore{
					DiskDeviceName: "test-device",
				},
			},
			errMessage: "Indexes can not be created on maps that have Tiered Storage enabled",
		},
		{
			name:        "LocalDevice does not exist",
			localDevice: hazelcastv1alpha1.LocalDeviceConfig{Name: "test-device-2"},
			mapSpec: hazelcastv1alpha1.MapSpec{
				InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
				TieredStore: &hazelcastv1alpha1.TieredStore{
					DiskDeviceName: "test-device",
				},
			},
			errMessage: "device with the name test-device does not exist",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			RegisterFailHandler(fail(t))
			h.Spec.LocalDevices = []hazelcastv1alpha1.LocalDeviceConfig{
				test.localDevice,
			}
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
