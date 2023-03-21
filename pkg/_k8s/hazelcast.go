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

// ListHazelcasts will look for Hazelcasts in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListHazelcasts(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []v1alpha1.Hazelcast {
	h, err := ListHazelcastsE(t, options, filters)
	require.NoError(t, err)
	return h
}

// ListHazelcastsE will look for Hazelcasts in the given namespace that match the given filters and return them.
func ListHazelcastsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]v1alpha1.Hazelcast, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	resp, err := clientset.HazelcastV1alpha1().Hazelcasts(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetHazelcast returns a Kubernetes Hazelcast resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetHazelcast(t testing.TestingT, options *KubectlOptions, HazelcastName string) *v1alpha1.Hazelcast {
	h, err := GetHazelcastE(t, options, HazelcastName)
	require.NoError(t, err)
	return h
}

// GetHazelcastE returns a Kubernetes Hazelcast resource in the provided namespace with the given name.
func GetHazelcastE(t testing.TestingT, options *KubectlOptions, HazelcastName string) (*v1alpha1.Hazelcast, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.HazelcastV1alpha1().Hazelcasts(options.Namespace).Get(context.Background(), HazelcastName, metav1.GetOptions{})
}

// WaitUntilHazelcastAvailable waits until the Hazelcast endpoint is ready to accept traffic.
func WaitUntilHazelcastAvailable(t testing.TestingT, options *KubectlOptions, HazelcastName string, retries int, sleepBetweenRetries time.Duration) {
	status := fmt.Sprintf("Wait for Hazelcast %s to be provisioned.", HazelcastName)
	message := retry.DoWithRetry(
		t,
		status,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			hz, err := GetHazelcastE(t, options, HazelcastName)
			if err != nil {
				return "", err
			}
			if !IsHazelcastAvailable(hz) {
				return "", NewHazelcastNotAvailableError(hz)
			}
			return "Hazelcast is now available", nil
		},
	)
	logger.Logf(t, message)
}

// IsHazelcastAvailable returns true if the Hazelcast endpoint is provisioned and available.
func IsHazelcastAvailable(hazelcast *v1alpha1.Hazelcast) bool {
	return hazelcast.Status.Phase == v1alpha1.Running
}

// HazelcastNotAvailable is returned when a Kubernetes Hazelcast is not yet available to accept traffic.
type HazelcastNotAvailable struct {
	Hazelcast *v1alpha1.Hazelcast
}

// Error is a simple function to return a formatted error message as a string
func (err HazelcastNotAvailable) Error() string {
	return fmt.Sprintf("Hazelcast %s is not available", err.Hazelcast.Name)
}

// NewHazelcastNotAvailableError returnes a HazelcastNotAvailable struct when Kubernetes deems a Hazelcast is not available
func NewHazelcastNotAvailableError(hazelcast *v1alpha1.Hazelcast) HazelcastNotAvailable {
	return HazelcastNotAvailable{hazelcast}
}
