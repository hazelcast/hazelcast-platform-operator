package k8s

import (
	"context"

	"github.com/gruntwork-io/terratest/modules/testing"
	"github.com/stretchr/testify/require"

	v1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListCronHotBackups will look for CronHotBackups in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListCronHotBackups(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []v1alpha1.CronHotBackup {
	h, err := ListCronHotBackupsE(t, options, filters)
	require.NoError(t, err)
	return h
}

// ListCronHotBackupsE will look for CronHotBackups in the given namespace that match the given filters and return them.
func ListCronHotBackupsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]v1alpha1.CronHotBackup, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	resp, err := clientset.HazelcastV1alpha1().CronHotBackups(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetCronHotBackup returns a Kubernetes CronHotBackup resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetCronHotBackup(t testing.TestingT, options *KubectlOptions, name string) *v1alpha1.CronHotBackup {
	h, err := GetCronHotBackupE(t, options, name)
	require.NoError(t, err)
	return h
}

// GetCronHotBackupE returns a Kubernetes CronHotBackup resource in the provided namespace with the given name.
func GetCronHotBackupE(t testing.TestingT, options *KubectlOptions, name string) (*v1alpha1.CronHotBackup, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.HazelcastV1alpha1().CronHotBackups(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
}
