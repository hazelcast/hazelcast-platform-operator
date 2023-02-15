package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/testing"
	"github.com/stretchr/testify/require"

	v1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListHotBackups will look for HotBackups in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListHotBackups(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []v1alpha1.HotBackup {
	h, err := ListHotBackupsE(t, options, filters)
	require.NoError(t, err)
	return h
}

// ListHotBackupsE will look for HotBackups in the given namespace that match the given filters and return them.
func ListHotBackupsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]v1alpha1.HotBackup, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	resp, err := clientset.HazelcastV1alpha1().HotBackups(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetHotBackup returns a Kubernetes HotBackup resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetHotBackup(t testing.TestingT, options *KubectlOptions, name string) *v1alpha1.HotBackup {
	h, err := GetHotBackupE(t, options, name)
	require.NoError(t, err)
	return h
}

// GetHotBackupE returns a Kubernetes HotBackup resource in the provided namespace with the given name.
func GetHotBackupE(t testing.TestingT, options *KubectlOptions, name string) (*v1alpha1.HotBackup, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.HazelcastV1alpha1().HotBackups(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// WaitUntilHotBackupAvailable waits until the HotBackup endpoint is ready to accept traffic.
func WaitUntilHotBackupAvailable(t testing.TestingT, options *KubectlOptions, name string, retries int, sleepBetweenRetries time.Duration) {
	status := fmt.Sprintf("Wait for HotBackup %s to be provisioned.", name)
	message := retry.DoWithRetry(
		t,
		status,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			hz, err := GetHotBackupE(t, options, name)
			if err != nil {
				return "", err
			}
			if !IsHotBackupAvailable(hz) {
				return "", NewHotBackupNotAvailableError(hz)
			}
			return "HotBackup is now available", nil
		},
	)
	logger.Logf(t, message)
}

// IsHotBackupAvailable returns true if the HotBackup endpoint is provisioned and available.
func IsHotBackupAvailable(hotBackup *v1alpha1.HotBackup) bool {
	return hotBackup.Status.State == v1alpha1.HotBackupSuccess
}

// HotBackupNotAvailable is returned when a Kubernetes HotBackup is not yet available to accept traffic.
type HotBackupNotAvailable struct {
	hotBackup *v1alpha1.HotBackup
}

// Error is a simple function to return a formatted error message as a string
func (err HotBackupNotAvailable) Error() string {
	return fmt.Sprintf("HotBackup %s is not available", err.hotBackup.Name)
}

// NewHotBackupNotAvailableError returnes a HotBackupNotAvailable struct when Kubernetes deems a HotBackup is not available
func NewHotBackupNotAvailableError(hotBackup *v1alpha1.HotBackup) HotBackupNotAvailable {
	return HotBackupNotAvailable{hotBackup}
}
