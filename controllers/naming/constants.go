package naming

import (
	corev1 "k8s.io/api/core/v1"
)

// Labels and label values
const (
	// Finalizer name used by operator
	Finalizer = "hazelcast.com/finalizer"
	// LicenseDataKey is a key used in k8s secret that holds the Hazelcast license
	LicenseDataKey = "license-key"
	// LicenseKeySecret default license key secret
	LicenseKeySecret = "hazelcast-license-key"
	// ServicePerPodLabelName set to true when the service is a Service per pod
	ServicePerPodLabelName         = "hazelcast.com/service-per-pod"
	ServicePerPodCountAnnotation   = "hazelcast.com/service-per-pod-count"
	ExposeExternallyAnnotation     = "hazelcast.com/expose-externally-member-access"
	LastSuccessfulConfigAnnotation = "hazelcast.com/last-successful-config"

	// PodNameLabel label that represents the name of the pod in the StatefulSet
	PodNameLabel = "statefulset.kubernetes.io/pod-name"
	// ApplicationNameLabel label for the name of the application
	ApplicationNameLabel = "app.kubernetes.io/name"
	// ApplicationInstanceNameLabel label for a unique name identifying the instance of an application
	ApplicationInstanceNameLabel = "app.kubernetes.io/instance"
	// ApplicationManagedByLabel label for the tool being used to manage the operation of an application
	ApplicationManagedByLabel = "app.kubernetes.io/managed-by"

	LabelValueTrue  = "true"
	LabelValueFalse = "false"

	OperatorName      = "hazelcast-platform-operator"
	Hazelcast         = "hazelcast"
	HazelcastPortName = "hazelcast-port"

	// ManagementCenter MC name
	ManagementCenter = "management-center"
	// Mancenter MC short name
	Mancenter = "mancenter"
	// MancenterStorageName storage name for MC
	MancenterStorageName = Mancenter + "-storage"
)

// Hazelcast default configurations
const (
	// DefaultHzPort Hazelcast default port
	DefaultHzPort = 5701
	// DefaultClusterSize default number of members of Hazelcast cluster
	DefaultClusterSize = 3
	// HazelcastRepo image repository for Hazelcast
	HazelcastRepo = "hazelcast/hazelcast"
	// HazelcastEERepo image repository for Hazelcast EE
	HazelcastEERepo = "hazelcast/hazelcast-enterprise"
	// HazelcastVersion version of Hazelcast image
	HazelcastVersion = "5.0.2"
	// HazelcastImagePullPolicy pull policy for Hazelcast Platform image
	HazelcastImagePullPolicy = corev1.PullIfNotPresent
)

// Management Center default configurations
const (
	// MCRepo image repository for Management Center
	MCRepo = "hazelcast/management-center"
	// MCVersion version of Management Center image
	MCVersion = "5.0.4"
	// MCImagePullPolicy pull policy for Management Center image
	MCImagePullPolicy = corev1.PullIfNotPresent
)
