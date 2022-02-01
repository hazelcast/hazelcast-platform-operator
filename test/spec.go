package test

import (
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type HazelcastSpecValues struct {
	ClusterSize     int32
	Repository      string
	Version         string
	LicenseKey      string
	ImagePullPolicy corev1.PullPolicy
}

func HazelcastSpec(values *HazelcastSpecValues, ee bool) hazelcastv1alpha1.HazelcastSpec {
	spec := hazelcastv1alpha1.HazelcastSpec{
		ClusterSize:     values.ClusterSize,
		Repository:      values.Repository,
		Version:         values.Version,
		ImagePullPolicy: values.ImagePullPolicy,
	}
	if ee {
		spec.LicenseKeySecret = values.LicenseKey
	}
	return spec
}

func CheckHazelcastCR(h *hazelcastv1alpha1.Hazelcast, expected *HazelcastSpecValues, ee bool) {
	Expect(h.Spec.ClusterSize).Should(Equal(expected.ClusterSize))
	Expect(h.Spec.Repository).Should(Equal(expected.Repository))
	Expect(h.Spec.Version).Should(Equal(expected.Version))
	Expect(h.Spec.ImagePullPolicy).Should(Equal(expected.ImagePullPolicy))
	if ee {
		Expect(h.Spec.LicenseKeySecret).Should(Equal(expected.LicenseKey))
	}
}

type MCSpecValues struct {
	Repository      string
	Version         string
	LicenseKey      string
	ImagePullPolicy corev1.PullPolicy
}

func ManagementCenterSpec(values *MCSpecValues, ee bool) hazelcastv1alpha1.ManagementCenterSpec {
	spec := hazelcastv1alpha1.ManagementCenterSpec{
		Repository:      values.Repository,
		Version:         values.Version,
		ImagePullPolicy: values.ImagePullPolicy,
		ExternalConnectivity: hazelcastv1alpha1.ExternalConnectivityConfiguration{
			Type: hazelcastv1alpha1.ExternalConnectivityTypeLoadBalancer,
		},
		HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{},
		Persistence: hazelcastv1alpha1.PersistenceConfiguration{
			StorageClass: nil,
		},
	}
	if ee {
		spec.LicenseKeySecret = values.LicenseKey
	}
	return spec
}

func CheckManagementCenterCR(mc *hazelcastv1alpha1.ManagementCenter, expected *MCSpecValues, ee bool) {
	Expect(mc.Spec.Repository).Should(Equal(expected.Repository))
	Expect(mc.Spec.Version).Should(Equal(expected.Version))
	Expect(mc.Spec.ImagePullPolicy).Should(Equal(expected.ImagePullPolicy))

	if ee {
		Expect(mc.Spec.LicenseKeySecret).Should(Equal(expected.LicenseKey))
	}
}
