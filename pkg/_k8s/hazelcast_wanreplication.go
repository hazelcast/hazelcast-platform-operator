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

// ListWanReplications will look for WanReplications in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListWanReplications(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []v1alpha1.WanReplication {
	h, err := ListWanReplicationsE(t, options, filters)
	require.NoError(t, err)
	return h
}

// ListWanReplicationsE will look for WanReplications in the given namespace that match the given filters and return them.
func ListWanReplicationsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]v1alpha1.WanReplication, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	resp, err := clientset.HazelcastV1alpha1().WanReplications(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetWanReplication returns a Kubernetes WanReplication resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetWanReplication(t testing.TestingT, options *KubectlOptions, name string) *v1alpha1.WanReplication {
	h, err := GetWanReplicationE(t, options, name)
	require.NoError(t, err)
	return h
}

// GetWanReplicationE returns a Kubernetes WanReplication resource in the provided namespace with the given name.
func GetWanReplicationE(t testing.TestingT, options *KubectlOptions, name string) (*v1alpha1.WanReplication, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.HazelcastV1alpha1().WanReplications(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// WaitUntilWanReplicationAvailable waits until the WanReplication endpoint is ready to accept traffic.
func WaitUntilWanReplicationAvailable(t testing.TestingT, options *KubectlOptions, name string, retries int, sleepBetweenRetries time.Duration) {
	status := fmt.Sprintf("Wait for WanReplication %s to be provisioned.", name)
	message := retry.DoWithRetry(
		t,
		status,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			hz, err := GetWanReplicationE(t, options, name)
			if err != nil {
				return "", err
			}
			if !IsWanReplicationAvailable(hz) {
				return "", NewWanReplicationNotAvailableError(hz)
			}
			return "WanReplication is now available", nil
		},
	)
	logger.Logf(t, message)
}

// IsWanReplicationAvailable returns true if the WanReplication endpoint is provisioned and available.
func IsWanReplicationAvailable(wanReplication *v1alpha1.WanReplication) bool {
	return wanReplication.Status.Status == v1alpha1.WanStatusSuccess
}

// WanReplicationNotAvailable is returned when a Kubernetes WanReplication is not yet available to accept traffic.
type WanReplicationNotAvailable struct {
	wanReplication *v1alpha1.WanReplication
}

// Error is a simple function to return a formatted error message as a string
func (err WanReplicationNotAvailable) Error() string {
	return fmt.Sprintf("WanReplication %s is not available", err.wanReplication.Name)
}

// NewWanReplicationNotAvailableError returnes a WanReplicationNotAvailable struct when Kubernetes deems a WanReplication is not available
func NewWanReplicationNotAvailableError(wanReplication *v1alpha1.WanReplication) WanReplicationNotAvailable {
	return WanReplicationNotAvailable{wanReplication}
}
