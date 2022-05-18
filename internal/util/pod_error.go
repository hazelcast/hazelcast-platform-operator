package util

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type PodError struct {
	Name         string
	Namespace    string
	Message      string
	Reason       string
	PodIp        string
	RestartCount int32
}

func NewPodError(pod *corev1.Pod) *PodError {
	return &PodError{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Message:   pod.Status.Message,
		PodIp:     pod.Status.PodIP,
	}
}

func NewPodErrorWithContainerStatus(pod *corev1.Pod, status corev1.ContainerStatus) *PodError {
	err := NewPodError(pod)
	err.RestartCount = status.RestartCount
	err.Reason = status.State.Waiting.Reason
	return err
}

func (e *PodError) Error() string {
	return fmt.Sprintf("pod %s in namespace %s failed for %s: %s", e.Name, e.Namespace, e.Message, e.Reason)
}

type PodErrors []*PodError

func (es PodErrors) Error() string {
	if len(es) == 0 {
		return ""
	}
	return fmt.Sprintf("multiple (%d) errors: %s", len(es), es[0].Error())
}
