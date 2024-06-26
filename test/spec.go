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

func HazelcastSpec(values *HazelcastSpecValues) hazelcastv1alpha1.HazelcastSpec {
	return hazelcastv1alpha1.HazelcastSpec{
		ClusterSize:          &values.ClusterSize,
		Repository:           values.Repository,
		Version:              values.Version,
		ImagePullPolicy:      values.ImagePullPolicy,
		LicenseKeySecretName: values.LicenseKey,
	}
}

func CheckHazelcastCR(h *hazelcastv1alpha1.Hazelcast, expected *HazelcastSpecValues) {
	Expect(*h.Spec.ClusterSize).Should(Equal(expected.ClusterSize))
	Expect(h.Spec.Repository).Should(Equal(expected.Repository))
	Expect(h.Spec.Version).Should(Equal(expected.Version))
	Expect(h.Spec.ImagePullPolicy).Should(Equal(expected.ImagePullPolicy))
	Expect(h.Spec.GetLicenseKeySecretName()).Should(Equal(expected.LicenseKey))
}

type MCSpecValues struct {
	Repository      string
	Version         string
	LicenseKey      string
	ImagePullPolicy corev1.PullPolicy
}

func ManagementCenterSpec(values *MCSpecValues) hazelcastv1alpha1.ManagementCenterSpec {
	return hazelcastv1alpha1.ManagementCenterSpec{
		Repository:           values.Repository,
		Version:              values.Version,
		ImagePullPolicy:      values.ImagePullPolicy,
		LicenseKeySecretName: values.LicenseKey,
	}
}

func CheckManagementCenterCR(mc *hazelcastv1alpha1.ManagementCenter, expected *MCSpecValues) {
	Expect(mc.Spec.Repository).Should(Equal(expected.Repository))
	Expect(mc.Spec.Version).Should(Equal(expected.Version))
	Expect(mc.Spec.ImagePullPolicy).Should(Equal(expected.ImagePullPolicy))
	Expect(mc.Spec.GetLicenseKeySecretName()).Should(Equal(expected.LicenseKey))
}
