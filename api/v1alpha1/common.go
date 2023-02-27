package v1alpha1

// InMemoryFormatType represents the format options for storing the data in the map/cache.
// +kubebuilder:validation:Enum=BINARY;OBJECT;NATIVE
type InMemoryFormatType string

const (
	// InMemoryFormatBinary Data will be stored in serialized binary format.
	InMemoryFormatBinary InMemoryFormatType = "BINARY"

	// InMemoryFormatObject Data will be stored in deserialized form.
	InMemoryFormatObject InMemoryFormatType = "OBJECT"

	// InMemoryFormatNative Data will be stored in the map that uses Hazelcast's High-Density Memory Store feature.
	InMemoryFormatNative InMemoryFormatType = "NATIVE"
)
