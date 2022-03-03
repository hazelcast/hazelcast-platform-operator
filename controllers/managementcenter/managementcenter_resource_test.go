package managementcenter

import (
	"testing"

	v1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

func Test_clusterAddCommand(t *testing.T) {
	tests := []struct {
		name string
		mc   *hazelcastv1alpha1.ManagementCenter
		want string
	}{
		{
			name: "No Cluster Defined",
			mc: &hazelcastv1alpha1.ManagementCenter{
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{},
				},
			},
			want: "",
		},
		{
			name: "One Cluster Defined",
			mc: &hazelcastv1alpha1.ManagementCenter{
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{
						{
							Name:    "dev",
							Address: "hazelcast",
						},
					},
				},
			},
			want: "./bin/mc-conf.sh cluster add --lenient=true -H /data -cn dev -ma hazelcast",
		},
		{
			name: "Two Clusters Defined",
			mc: &hazelcastv1alpha1.ManagementCenter{
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{
						{
							Name:    "dev",
							Address: "hazelcast",
						},
						{
							Name:    "prod",
							Address: "hazelcast-prod",
						},
					},
				},
			},
			want: "./bin/mc-conf.sh cluster add --lenient=true -H /data -cn dev -ma hazelcast && ./bin/mc-conf.sh cluster add --lenient=true -H /data -cn prod -ma hazelcast-prod",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clusterAddCommand(tt.mc); got != tt.want {
				t.Errorf("clusterAddCommand() = %v, want %v", got, tt.want)
			}
		})
	}

}

func reconcilerWithCR(mc *hazelcastv1alpha1.ManagementCenter) ManagementCenterReconciler {
	scheme, _ := hazelcastv1alpha1.SchemeBuilder.
		Register(&hazelcastv1alpha1.ManagementCenter{}, &hazelcastv1alpha1.ManagementCenterList{}, &v1.ClusterRole{}, &v1.ClusterRoleBinding{}).
		Build()
	return ManagementCenterReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(mc).Build(),
	}
}

func assertMCSpecEquals(t *testing.T, actual hazelcastv1alpha1.ManagementCenterSpec, expected hazelcastv1alpha1.ManagementCenterSpec) {
	if actual.Repository != expected.Repository || actual.Version != expected.Version {
		t.Errorf("ManagementCenterSpec = %v, want %v", actual, expected)
	}
}
