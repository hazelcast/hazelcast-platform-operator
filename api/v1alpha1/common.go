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

type EventJournal struct {
	// Enabled defines if event journal is enabled.
	// +kubebuilder:default:=false
	Enabled bool `json:"enabled,omitempty"`
	// Capacity sets the capacity of the ringbuffer underlying the event journal.
	// +kubebuilder:default:=10000
	Capacity int32 `json:"capacity,omitempty"`
	// TimeToLiveSeconds indicates how long the items remain in the event journal before they are expired.
	// +kubebuilder:default:=0
	TimeToLiveSeconds int32 `json:"timeToLiveSeconds,omitempty"`
}
