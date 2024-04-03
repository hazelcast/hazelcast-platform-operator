package v1alpha1

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CacheSpec defines the desired state of Cache
// It cannot be updated after the Cache is created
type CacheSpec struct {
	DataStructureSpec `json:",inline"`

	// Class name of the key type
	// +optional
	KeyType string `json:"keyType,omitempty"`

	// Class name of the value type
	// +optional
	ValueType string `json:"valueType,omitempty"`

	// When enabled, cache data will be persisted.
	// +kubebuilder:default:=false
	// +optional
	PersistenceEnabled bool `json:"persistenceEnabled"`

	// InMemoryFormat specifies in which format data will be stored in your cache
	// +kubebuilder:default:=BINARY
	// +optional
	InMemoryFormat InMemoryFormatType `json:"inMemoryFormat,omitempty"`

	// EventJournal specifies event journal configuration of the Cache
	// +optional
	EventJournal *EventJournal `json:"eventJournal,omitempty"`
}

// CacheStatus defines the observed state of Cache
type CacheStatus struct {
	DataStructureStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the Cache Config"
// +kubebuilder:printcolumn:name="Hazelcast-Resource",type="string",priority=1,JSONPath=".spec.hazelcastResourceName",description="Name of the Hazelcast resource that this resource is created for"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current Cache Config"
// +kubebuilder:resource:shortName=ch

// Cache is the Schema for the caches API
type Cache struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec CacheSpec `json:"spec"`
	// +optional
	Status CacheStatus `json:"status,omitempty"`
}

func (c *Cache) GetKind() string {
	return c.Kind
}

func (c *Cache) GetDSName() string {
	if c.Spec.Name != "" {
		return c.Spec.Name
	}
	return c.Name
}

func (c *Cache) GetHZResourceName() string {
	return c.Spec.HazelcastResourceName
}

func (c *Cache) GetStatus() *DataStructureStatus {
	return &c.Status.DataStructureStatus
}

func (c *Cache) GetSpec() (string, error) {
	cs, err := json.Marshal(c.Spec)
	if err != nil {
		return "", fmt.Errorf("error marshaling %v as JSON: %w", c.Kind, err)
	}
	return string(cs), nil
}

func (c *Cache) SetSpec(spec string) error {
	if err := json.Unmarshal([]byte(spec), &c.Spec); err != nil {
		return err
	}
	return nil
}

func (c *Cache) ValidateSpecCurrent(h *Hazelcast) error {
	return ValidateCacheSpecCurrent(c, h)
}

func (c *Cache) ValidateSpecCreate() error {
	return validateCacheSpecCreate(c)
}

func (c *Cache) ValidateSpecUpdate() error {
	return validateCacheSpecUpdate(c)
}

//+kubebuilder:object:root=true

// CacheList contains a list of Cache
type CacheList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cache `json:"items"`
}

func (cl *CacheList) GetItems() []client.Object {
	l := make([]client.Object, 0, len(cl.Items))
	for i := range cl.Items {
		l = append(l, &cl.Items[i])
	}
	return l
}

func init() {
	SchemeBuilder.Register(&Cache{}, &CacheList{})
}
