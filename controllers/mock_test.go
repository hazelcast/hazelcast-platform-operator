package controllers

import (
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Centralized mock objects for use in tests
var (
	hzWithoutSpec = hazelcastv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hazelcast-test",
			Namespace: "hazelcast-enterprise-operator",
		},
	}

	hzWithLicenseKeySpec = hazelcastv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hazelcast-test",
			Namespace: "hazelcast-enterprise-operator",
		},
		Spec: hazelcastv1alpha1.HazelcastSpec{
			LicenseKeySecret: "hazelcast-test-license-key",
		},
	}

	licenseKeyString = "PLATFORM#9Nodes#ABCDEF"

	licenseKeySecret = v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hazelcast-test-license-key",
			Namespace: "hazelcast-enterprise-operator",
		},
		Data: createLicenseKeySecretData(),
	}
)

func createLicenseKeySecretData() map[string][]byte {
	licenseKeyData := make(map[string][]byte)
	licenseKeyData[licenseDataKey] = []byte(licenseKeyString)

	return licenseKeyData
}
