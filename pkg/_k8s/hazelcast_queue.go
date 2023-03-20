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

// ListQueues will look for Queues in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListQueues(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []v1alpha1.Queue {
	h, err := ListQueuesE(t, options, filters)
	require.NoError(t, err)
	return h
}

// ListQueuesE will look for Queues in the given namespace that match the given filters and return them.
func ListQueuesE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]v1alpha1.Queue, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	resp, err := clientset.HazelcastV1alpha1().Queues(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetQueue returns a Kubernetes Queue resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetQueue(t testing.TestingT, options *KubectlOptions, name string) *v1alpha1.Queue {
	h, err := GetQueueE(t, options, name)
	require.NoError(t, err)
	return h
}

// GetQueueE returns a Kubernetes Queue resource in the provided namespace with the given name.
func GetQueueE(t testing.TestingT, options *KubectlOptions, name string) (*v1alpha1.Queue, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.HazelcastV1alpha1().Queues(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// WaitUntilQueueAvailable waits until the Queue endpoint is ready to accept traffic.
func WaitUntilQueueAvailable(t testing.TestingT, options *KubectlOptions, name string, retries int, sleepBetweenRetries time.Duration) {
	status := fmt.Sprintf("Wait for Queue %s to be provisioned.", name)
	message := retry.DoWithRetry(
		t,
		status,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			hz, err := GetQueueE(t, options, name)
			if err != nil {
				return "", err
			}
			if !IsQueueAvailable(hz) {
				return "", NewQueueNotAvailableError(hz)
			}
			return "Queue is now available", nil
		},
	)
	logger.Logf(t, message)
}

// IsQueueAvailable returns true if the Queue endpoint is provisioned and available.
func IsQueueAvailable(queue *v1alpha1.Queue) bool {
	return queue.Status.DataStructureStatus.State == v1alpha1.DataStructureSuccess
}

// QueueNotAvailable is returned when a Kubernetes Queue is not yet available to accept traffic.
type QueueNotAvailable struct {
	queue *v1alpha1.Queue
}

// Error is a simple function to return a formatted error message as a string
func (err QueueNotAvailable) Error() string {
	return fmt.Sprintf("Queue %s is not available", err.queue.Name)
}

// NewQueueNotAvailableError returnes a QueueNotAvailable struct when Kubernetes deems a Queue is not available
func NewQueueNotAvailableError(queue *v1alpha1.Queue) QueueNotAvailable {
	return QueueNotAvailable{queue}
}
