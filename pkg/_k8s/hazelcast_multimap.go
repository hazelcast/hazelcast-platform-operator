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

// ListMultiMaps will look for MultiMaps in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListMultiMaps(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []v1alpha1.MultiMap {
	h, err := ListMultiMapsE(t, options, filters)
	require.NoError(t, err)
	return h
}

// ListMultiMapsE will look for MultiMaps in the given namespace that match the given filters and return them.
func ListMultiMapsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]v1alpha1.MultiMap, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	resp, err := clientset.HazelcastV1alpha1().MultiMaps(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetMultiMap returns a Kubernetes MultiMap resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetMultiMap(t testing.TestingT, options *KubectlOptions, name string) *v1alpha1.MultiMap {
	h, err := GetMultiMapE(t, options, name)
	require.NoError(t, err)
	return h
}

// GetMultiMapE returns a Kubernetes MultiMap resource in the provided namespace with the given name.
func GetMultiMapE(t testing.TestingT, options *KubectlOptions, name string) (*v1alpha1.MultiMap, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.HazelcastV1alpha1().MultiMaps(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// WaitUntilMultiMapAvailable waits until the MultiMap endpoint is ready to accept traffic.
func WaitUntilMultiMapAvailable(t testing.TestingT, options *KubectlOptions, name string, retries int, sleepBetweenRetries time.Duration) {
	status := fmt.Sprintf("Wait for MultiMap %s to be provisioned.", name)
	message := retry.DoWithRetry(
		t,
		status,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			hz, err := GetMultiMapE(t, options, name)
			if err != nil {
				return "", err
			}
			if !IsMultiMapAvailable(hz) {
				return "", NewMultiMapNotAvailableError(hz)
			}
			return "MultiMap is now available", nil
		},
	)
	logger.Logf(t, message)
}

// IsMultiMapAvailable returns true if the MultiMap endpoint is provisioned and available.
func IsMultiMapAvailable(multiMap *v1alpha1.MultiMap) bool {
	return multiMap.Status.DataStructureStatus.State == v1alpha1.DataStructureSuccess
}

// MultiMapNotAvailable is returned when a Kubernetes MultiMap is not yet available to accept traffic.
type MultiMapNotAvailable struct {
	multiMap *v1alpha1.MultiMap
}

// Error is a simple function to return a formatted error message as a string
func (err MultiMapNotAvailable) Error() string {
	return fmt.Sprintf("MultiMap %s is not available", err.multiMap.Name)
}

// NewMultiMapNotAvailableError returnes a MultiMapNotAvailable struct when Kubernetes deems a MultiMap is not available
func NewMultiMapNotAvailableError(multiMap *v1alpha1.MultiMap) MultiMapNotAvailable {
	return MultiMapNotAvailable{multiMap}
}
