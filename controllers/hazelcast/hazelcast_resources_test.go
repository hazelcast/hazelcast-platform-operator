package hazelcast

import (
	"testing"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
)

func Test_mergeHazelcastSpecs(t *testing.T) {
	tests := []struct {
		name   string
		target *hazelcastv1alpha1.HazelcastSpec
		want   hazelcastv1alpha1.HazelcastSpec
	}{
		{
			name:   "Empty hazelcast repository",
			target: &hazelcastv1alpha1.HazelcastSpec{ClusterSize: n.DefaultClusterSize, Version: n.HazelcastVersion, LicenseKeySecret: n.LicenseKeySecret},
			want:   hazelcastv1alpha1.HazelcastSpec{ClusterSize: n.DefaultClusterSize, Repository: n.HazelcastRepo, Version: n.HazelcastVersion, LicenseKeySecret: n.LicenseKeySecret},
		},
		{
			name:   "Empty hazelcast version",
			target: &hazelcastv1alpha1.HazelcastSpec{ClusterSize: n.DefaultClusterSize, Repository: n.HazelcastRepo, LicenseKeySecret: n.LicenseKeySecret},
			want:   hazelcastv1alpha1.HazelcastSpec{ClusterSize: n.DefaultClusterSize, Repository: n.HazelcastRepo, LicenseKeySecret: n.LicenseKeySecret, Version: n.HazelcastVersion},
		},
		{
			name:   "Empty license key secret",
			target: &hazelcastv1alpha1.HazelcastSpec{ClusterSize: n.DefaultClusterSize, Repository: n.HazelcastRepo, Version: n.HazelcastVersion},
			want:   hazelcastv1alpha1.HazelcastSpec{ClusterSize: n.DefaultClusterSize, Repository: n.HazelcastRepo, Version: n.HazelcastVersion},
		},
		{
			name:   "Empty cluster size",
			target: &hazelcastv1alpha1.HazelcastSpec{LicenseKeySecret: n.LicenseKeySecret, Repository: n.HazelcastRepo, Version: n.HazelcastVersion},
			want:   hazelcastv1alpha1.HazelcastSpec{LicenseKeySecret: n.LicenseKeySecret, ClusterSize: n.DefaultClusterSize, Repository: n.HazelcastRepo, Version: n.HazelcastVersion},
		},
		{
			name:   "Non empty hazelcast repository",
			target: &hazelcastv1alpha1.HazelcastSpec{Repository: "myorg/hazelcast"},
			want:   hazelcastv1alpha1.HazelcastSpec{Repository: "myorg/hazelcast", ClusterSize: n.DefaultClusterSize, Version: n.HazelcastVersion},
		},
		{
			name:   "Non empty hazelcast version",
			target: &hazelcastv1alpha1.HazelcastSpec{Version: "4.2"},
			want:   hazelcastv1alpha1.HazelcastSpec{Version: "4.2", ClusterSize: n.DefaultClusterSize, Repository: n.HazelcastRepo},
		},
		{
			name:   "Non empty license key secret",
			target: &hazelcastv1alpha1.HazelcastSpec{LicenseKeySecret: "license-key-secret"},
			want:   hazelcastv1alpha1.HazelcastSpec{LicenseKeySecret: "license-key-secret", ClusterSize: n.DefaultClusterSize, Repository: n.HazelcastRepo, Version: n.HazelcastVersion},
		},
		{
			name:   "Non empty cluster size",
			target: &hazelcastv1alpha1.HazelcastSpec{ClusterSize: 5},
			want:   hazelcastv1alpha1.HazelcastSpec{ClusterSize: 5, Repository: n.HazelcastRepo, Version: n.HazelcastVersion},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyDefaultHazelcastSpecs(tt.target)
			if *tt.target != tt.want {
				t.Errorf("HazelcastSpec = %v, want %v", *tt.target, tt.want)
			}
		})
	}
}
