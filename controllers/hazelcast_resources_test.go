package controllers

import (
	"context"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

func TestHazelcastResources_EnsureLicenseKeyReadProperly(t *testing.T) {
	// objects to track in the fake client
	objs := []runtime.Object{
		&hzWithLicenseKeySpec,
		&licenseKeySecret,
	}

	r := buildReconcileWithFakeClientWithMocks(objs)
	// mock request to simulate Reconcile() being called on an event for a watched resource
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      hzWithLicenseKeySpec.Name,
			Namespace: hzWithLicenseKeySpec.Namespace,
		},
	}

	ctx := context.Background()
	_, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	sts := &appsv1.StatefulSet{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: hzWithLicenseKeySpec.Name, Namespace: hzWithLicenseKeySpec.Namespace}, sts)
	if err != nil {
		t.Errorf("TestReconcileHazelcast get StatefulSet error = %v", err)
		return
	}

	assert.Equal(t, licenseKeyString, getLicenseKeyFromStatefulSetSpec(sts))
}

func getLicenseKeyFromStatefulSetSpec(sts *appsv1.StatefulSet) string {
	for _, envVar := range sts.Spec.Template.Spec.Containers[0].Env {
		if envVar.Name == "HZ_LICENSEKEY" {
			return envVar.Value
		}
	}
	return ""
}
