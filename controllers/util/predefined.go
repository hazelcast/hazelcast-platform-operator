package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Predefined interface {
	PredefinedLabels() map[string]string
	PredefinedMetadata() metav1.ObjectMeta
	ExternalAddressEnabled() bool
}
