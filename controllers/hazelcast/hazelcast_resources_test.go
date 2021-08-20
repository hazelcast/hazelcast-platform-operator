package hazelcast

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_isStatefulSetReady(t *testing.T) {
	tests := []struct {
		name string
		sts  *appsv1.StatefulSet
		want bool
	}{
		{
			name: "StatefulSet is ready",
			sts:  statefulSet(3, 3, 1, 1),
			want: true,
		},
		{
			name: "Not all the replicas are updated in StatefulSet ",
			sts:  statefulSet(2, 3, 1, 1),
			want: false,
		},
		{
			name: "Not all the replicas are ready in StatefulSet ",
			sts:  statefulSet(3, 1, 1, 1),
			want: false,
		},
		{
			name: "StatefulSet of the older generation should not be ready",
			sts:  statefulSet(3, 3, 2, 1),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isStatefulSetReady(tt.sts, 3); got != tt.want {
				t.Errorf("isStatefulSetReady() = %v, want %v", got, tt.want)
			}
		})
	}

}

func statefulSet(updatedReplicas int32, readyReplicas int32, observedGeneration int64, generation int64) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		Status: appsv1.StatefulSetStatus{
			UpdatedReplicas:    updatedReplicas,
			ReadyReplicas:      readyReplicas,
			ObservedGeneration: observedGeneration,
		},
		ObjectMeta: v1.ObjectMeta{
			Generation: generation,
		},
	}
}
