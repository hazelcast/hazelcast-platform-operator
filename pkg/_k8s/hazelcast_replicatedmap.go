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

// ListReplicatedMaps will look for ReplicatedMaps in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListReplicatedMaps(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []v1alpha1.ReplicatedMap {
	h, err := ListReplicatedMapsE(t, options, filters)
	require.NoError(t, err)
	return h
}

// ListReplicatedMapsE will look for ReplicatedMaps in the given namespace that match the given filters and return them.
func ListReplicatedMapsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]v1alpha1.ReplicatedMap, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	resp, err := clientset.HazelcastV1alpha1().ReplicatedMaps(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetReplicatedMap returns a Kubernetes ReplicatedMap resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetReplicatedMap(t testing.TestingT, options *KubectlOptions, name string) *v1alpha1.ReplicatedMap {
	h, err := GetReplicatedMapE(t, options, name)
	require.NoError(t, err)
	return h
}

// GetReplicatedMapE returns a Kubernetes ReplicatedMap resource in the provided namespace with the given name.
func GetReplicatedMapE(t testing.TestingT, options *KubectlOptions, name string) (*v1alpha1.ReplicatedMap, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.HazelcastV1alpha1().ReplicatedMaps(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// WaitUntilReplicatedMapAvailable waits until the ReplicatedMap endpoint is ready to accept traffic.
func WaitUntilReplicatedMapAvailable(t testing.TestingT, options *KubectlOptions, name string, retries int, sleepBetweenRetries time.Duration) {
	status := fmt.Sprintf("Wait for ReplicatedMap %s to be provisioned.", name)
	message := retry.DoWithRetry(
		t,
		status,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			hz, err := GetReplicatedMapE(t, options, name)
			if err != nil {
				return "", err
			}
			if !IsReplicatedMapAvailable(hz) {
				return "", NewReplicatedMapNotAvailableError(hz)
			}
			return "ReplicatedMap is now available", nil
		},
	)
	logger.Logf(t, message)
}

// IsReplicatedMapAvailable returns true if the ReplicatedMap endpoint is provisioned and available.
func IsReplicatedMapAvailable(replicatedMap *v1alpha1.ReplicatedMap) bool {
	return replicatedMap.Status.DataStructureStatus.State == v1alpha1.DataStructureSuccess
}

// ReplicatedMapNotAvailable is returned when a Kubernetes ReplicatedMap is not yet available to accept traffic.
type ReplicatedMapNotAvailable struct {
	replicatedMap *v1alpha1.ReplicatedMap
}

// Error is a simple function to return a formatted error message as a string
func (err ReplicatedMapNotAvailable) Error() string {
	return fmt.Sprintf("ReplicatedMap %s is not available", err.replicatedMap.Name)
}

// NewReplicatedMapNotAvailableError returnes a ReplicatedMapNotAvailable struct when Kubernetes deems a ReplicatedMap is not available
func NewReplicatedMapNotAvailableError(replicatedMap *v1alpha1.ReplicatedMap) ReplicatedMapNotAvailable {
	return ReplicatedMapNotAvailable{replicatedMap}
}
