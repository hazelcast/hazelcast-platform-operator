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

// ListMaps will look for Maps in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListMaps(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []v1alpha1.Map {
	h, err := ListMapsE(t, options, filters)
	require.NoError(t, err)
	return h
}

// ListMapsE will look for Maps in the given namespace that match the given filters and return them.
func ListMapsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]v1alpha1.Map, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	resp, err := clientset.HazelcastV1alpha1().Maps(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetMap returns a Kubernetes Map resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetMap(t testing.TestingT, options *KubectlOptions, name string) *v1alpha1.Map {
	h, err := GetMapE(t, options, name)
	require.NoError(t, err)
	return h
}

// GetMapE returns a Kubernetes Map resource in the provided namespace with the given name.
func GetMapE(t testing.TestingT, options *KubectlOptions, name string) (*v1alpha1.Map, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.HazelcastV1alpha1().Maps(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// WaitUntilMapAvailable waits until the Map endpoint is ready to accept traffic.
func WaitUntilMapAvailable(t testing.TestingT, options *KubectlOptions, name string, retries int, sleepBetweenRetries time.Duration) {
	status := fmt.Sprintf("Wait for Map %s to be provisioned.", name)
	message := retry.DoWithRetry(
		t,
		status,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			hz, err := GetMapE(t, options, name)
			if err != nil {
				return "", err
			}
			if !IsMapAvailable(hz) {
				return "", NewMapNotAvailableError(hz)
			}
			return "Map is now available", nil
		},
	)
	logger.Logf(t, message)
}

// IsMapAvailable returns true if the Map endpoint is provisioned and available.
func IsMapAvailable(m *v1alpha1.Map) bool {
	return m.Status.State == v1alpha1.MapSuccess
}

// MapNotAvailable is returned when a Kubernetes Map is not yet available to accept traffic.
type MapNotAvailable struct {
	m *v1alpha1.Map
}

// Error is a simple function to return a formatted error message as a string
func (err MapNotAvailable) Error() string {
	return fmt.Sprintf("Map %s is not available", err.m.Name)
}

// NewMapNotAvailableError returnes a MapNotAvailable struct when Kubernetes deems a Map is not available
func NewMapNotAvailableError(m *v1alpha1.Map) MapNotAvailable {
	return MapNotAvailable{m}
}
