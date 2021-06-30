package controllers

import (
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"context"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

func TestHazelcastReconcile(t *testing.T) {

	type fields struct {
		scheme *runtime.Scheme
		objs   []runtime.Object
	}

	type args struct {
		hz hazelcastv1alpha1.Hazelcast
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Should work with default values",
			fields: fields{
				objs: []runtime.Object{&hzWithoutSpec},
			},
			args: args{
				hz: hzWithoutSpec,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			h := buildReconcileWithFakeClientWithMocks(tt.fields.objs)

			// mock request to simulate Reconcile() being called on an event for a watched resource
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.args.hz.Name,
					Namespace: tt.args.hz.Namespace,
				},
			}

			ctx := context.Background()

			_, err := h.Reconcile(ctx, req)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestReconcileDatabase reconcile: error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			clusterRole := &rbacv1.ClusterRole{}
			err = h.Client.Get(ctx, client.ObjectKey{Name: tt.args.hz.Name}, clusterRole)
			if err != nil {
				t.Errorf("TestReconcileHazelcast get ClusterRole error = %v", err)
				return
			}

			serviceAccount := &corev1.ServiceAccount{}
			err = h.Client.Get(ctx, types.NamespacedName{Name: tt.args.hz.Name, Namespace: tt.args.hz.Namespace}, serviceAccount)
			if err != nil {
				t.Errorf("TestReconcileHazelcast get ServiceAccount error = %v", err)
				return
			}

			roleBinding := &rbacv1.RoleBinding{}
			err = h.Client.Get(ctx, types.NamespacedName{Name: tt.args.hz.Name, Namespace: tt.args.hz.Namespace}, roleBinding)
			if err != nil {
				t.Errorf("TestReconcileHazelcast get RoleBinding error = %v", err)
				return
			}

			service := &corev1.Service{}
			err = h.Client.Get(ctx, types.NamespacedName{Name: tt.args.hz.Name, Namespace: tt.args.hz.Namespace}, service)
			if err != nil {
				t.Errorf("TestReconcileHazelcast get Service error = %v", err)
				return
			}

			sts := &appsv1.StatefulSet{}
			err = h.Client.Get(ctx, types.NamespacedName{Name: tt.args.hz.Name, Namespace: tt.args.hz.Namespace}, sts)
			if err != nil {
				t.Errorf("TestReconcileHazelcast get StatefulSet error = %v", err)
				return
			}
		})
	}
}

func TestHazelcastReconcile_EnsureFinalizerAdded(t *testing.T) {
	// objects to track in the fake client
	objs := []runtime.Object{
		&hzWithoutSpec,
	}

	r := buildReconcileWithFakeClientWithMocks(objs)
	// mock request to simulate Reconcile() being called on an event for a watched resource
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      hzWithoutSpec.Name,
			Namespace: hzWithoutSpec.Namespace,
		},
	}

	ctx := context.Background()
	_, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	h := &hazelcastv1alpha1.Hazelcast{}
	r.Client.Get(ctx, types.NamespacedName{Name: hzWithoutSpec.Name, Namespace: hzWithoutSpec.Namespace}, h)
	if err != nil {
		t.Errorf("get Hazelcast CR error = %v", err)
		return
	}

	assert.Equal(t, finalizer, h.ObjectMeta.Finalizers[0])
}

func TestHazelcastReconcile_EnsureClusterRoleRemovedWithCR(t *testing.T) {

}
