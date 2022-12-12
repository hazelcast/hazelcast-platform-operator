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
}

// CacheStatus defines the observed state of Cache
type CacheStatus struct {
	DataStructureStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the Cache Config"
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

func (c *Cache) GetStatus() DataStructureConfigState {
	return c.Status.State
}

func (c *Cache) GetMemberStatuses() map[string]DataStructureConfigState {
	return c.Status.MemberStatuses
}

func (c *Cache) SetStatus(state DataStructureConfigState, msg string, members map[string]DataStructureConfigState) {
	c.Status.State = state
	c.Status.Message = msg
	c.Status.MemberStatuses = members
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
