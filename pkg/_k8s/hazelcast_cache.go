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

// ListCaches will look for Caches in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListCaches(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []v1alpha1.Cache {
	h, err := ListCachesE(t, options, filters)
	require.NoError(t, err)
	return h
}

// ListCachesE will look for Caches in the given namespace that match the given filters and return them.
func ListCachesE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]v1alpha1.Cache, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	resp, err := clientset.HazelcastV1alpha1().Caches(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetCache returns a Kubernetes Cache resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetCache(t testing.TestingT, options *KubectlOptions, name string) *v1alpha1.Cache {
	h, err := GetCacheE(t, options, name)
	require.NoError(t, err)
	return h
}

// GetCacheE returns a Kubernetes Cache resource in the provided namespace with the given name.
func GetCacheE(t testing.TestingT, options *KubectlOptions, name string) (*v1alpha1.Cache, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.HazelcastV1alpha1().Caches(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// WaitUntilCacheAvailable waits until the Cache endpoint is ready to accept traffic.
func WaitUntilCacheAvailable(t testing.TestingT, options *KubectlOptions, name string, retries int, sleepBetweenRetries time.Duration) {
	status := fmt.Sprintf("Wait for Cache %s to be provisioned.", name)
	message := retry.DoWithRetry(
		t,
		status,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			hz, err := GetCacheE(t, options, name)
			if err != nil {
				return "", err
			}
			if !IsCacheAvailable(hz) {
				return "", NewCacheNotAvailableError(hz)
			}
			return "Cache is now available", nil
		},
	)
	logger.Logf(t, message)
}

// IsCacheAvailable returns true if the Cache endpoint is provisioned and available.
func IsCacheAvailable(cache *v1alpha1.Cache) bool {
	return cache.Status.DataStructureStatus.State == v1alpha1.DataStructureSuccess
}

// CacheNotAvailable is returned when a Kubernetes Cache is not yet available to accept traffic.
type CacheNotAvailable struct {
	cache *v1alpha1.Cache
}

// Error is a simple function to return a formatted error message as a string
func (err CacheNotAvailable) Error() string {
	return fmt.Sprintf("Cache %s is not available", err.cache.Name)
}

// NewCacheNotAvailableError returnes a CacheNotAvailable struct when Kubernetes deems a Cache is not available
func NewCacheNotAvailableError(cache *v1alpha1.Cache) CacheNotAvailable {
	return CacheNotAvailable{cache}
}
