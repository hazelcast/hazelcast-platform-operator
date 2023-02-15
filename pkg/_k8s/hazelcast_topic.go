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

// ListTopics will look for Topics in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListTopics(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []v1alpha1.Topic {
	h, err := ListTopicsE(t, options, filters)
	require.NoError(t, err)
	return h
}

// ListTopicsE will look for Topics in the given namespace that match the given filters and return them.
func ListTopicsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]v1alpha1.Topic, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	resp, err := clientset.HazelcastV1alpha1().Topics(options.Namespace).List(context.Background(), filters)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetTopic returns a Kubernetes Topic resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetTopic(t testing.TestingT, options *KubectlOptions, name string) *v1alpha1.Topic {
	h, err := GetTopicE(t, options, name)
	require.NoError(t, err)
	return h
}

// GetTopicE returns a Kubernetes Topic resource in the provided namespace with the given name.
func GetTopicE(t testing.TestingT, options *KubectlOptions, name string) (*v1alpha1.Topic, error) {
	clientset, err := GetHazelcastClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}
	return clientset.HazelcastV1alpha1().Topics(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// WaitUntilTopicAvailable waits until the Topic endpoint is ready to accept traffic.
func WaitUntilTopicAvailable(t testing.TestingT, options *KubectlOptions, name string, retries int, sleepBetweenRetries time.Duration) {
	status := fmt.Sprintf("Wait for Topic %s to be provisioned.", name)
	message := retry.DoWithRetry(
		t,
		status,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			hz, err := GetTopicE(t, options, name)
			if err != nil {
				return "", err
			}
			if !IsTopicAvailable(hz) {
				return "", NewTopicNotAvailableError(hz)
			}
			return "Topic is now available", nil
		},
	)
	logger.Logf(t, message)
}

// IsTopicAvailable returns true if the Topic endpoint is provisioned and available.
func IsTopicAvailable(topic *v1alpha1.Topic) bool {
	return topic.Status.DataStructureStatus.State == v1alpha1.DataStructureSuccess
}

// TopicNotAvailable is returned when a Kubernetes Topic is not yet available to accept traffic.
type TopicNotAvailable struct {
	topic *v1alpha1.Topic
}

// Error is a simple function to return a formatted error message as a string
func (err TopicNotAvailable) Error() string {
	return fmt.Sprintf("Topic %s is not available", err.topic.Name)
}

// NewTopicNotAvailableError returnes a TopicNotAvailable struct when Kubernetes deems a Topic is not available
func NewTopicNotAvailableError(topic *v1alpha1.Topic) TopicNotAvailable {
	return TopicNotAvailable{topic}
}
