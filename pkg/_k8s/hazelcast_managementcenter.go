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

// ListManagementCenters will look for ManagementCenters in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListManagementCenters(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []v1alpha1.ManagementCenter {
	h, err := ListManagementCentersE(t, options, filters)
	require.NoError(t, err)
	return h
}

// ListManagementCentersE will look for ManagementCenters in the given namespace that match the given filters and return them.
func ListManagementCentersE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]v1alpha1.ManagementCenter, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	resp, err := clientset.HazelcastV1alpha1().ManagementCenters(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetManagementCenter returns a Kubernetes ManagementCenter resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetManagementCenter(t testing.TestingT, options *KubectlOptions, name string) *v1alpha1.ManagementCenter {
	h, err := GetManagementCenterE(t, options, name)
	require.NoError(t, err)
	return h
}

// GetManagementCenterE returns a Kubernetes ManagementCenter resource in the provided namespace with the given name.
func GetManagementCenterE(t testing.TestingT, options *KubectlOptions, name string) (*v1alpha1.ManagementCenter, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.HazelcastV1alpha1().ManagementCenters(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// WaitUntilManagementCenterAvailable waits until the ManagementCenter endpoint is ready to accept traffic.
func WaitUntilManagementCenterAvailable(t testing.TestingT, options *KubectlOptions, name string, retries int, sleepBetweenRetries time.Duration) {
	status := fmt.Sprintf("Wait for ManagementCenter %s to be provisioned.", name)
	message := retry.DoWithRetry(
		t,
		status,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			hz, err := GetManagementCenterE(t, options, name)
			if err != nil {
				return "", err
			}
			if !IsManagementCenterAvailable(hz) {
				return "", NewManagementCenterNotAvailableError(hz)
			}
			return "ManagementCenter is now available", nil
		},
	)
	logger.Logf(t, message)
}

// IsManagementCenterAvailable returns true if the ManagementCenter endpoint is provisioned and available.
func IsManagementCenterAvailable(managementCenter *v1alpha1.ManagementCenter) bool {
	return managementCenter.Status.Phase == v1alpha1.Running
}

// ManagementCenterNotAvailable is returned when a Kubernetes ManagementCenter is not yet available to accept traffic.
type ManagementCenterNotAvailable struct {
	managementCenter *v1alpha1.ManagementCenter
}

// Error is a simple function to return a formatted error message as a string
func (err ManagementCenterNotAvailable) Error() string {
	return fmt.Sprintf("ManagementCenter %s is not available", err.managementCenter.Name)
}

// NewManagementCenterNotAvailableError returnes a ManagementCenterNotAvailable struct when Kubernetes deems a ManagementCenter is not available
func NewManagementCenterNotAvailableError(managementCenter *v1alpha1.ManagementCenter) ManagementCenterNotAvailable {
	return ManagementCenterNotAvailable{managementCenter}
}
