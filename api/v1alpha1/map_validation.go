package v1alpha1

import (
	"encoding/json"
	"fmt"
	"reflect"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mapValidator struct {
	datastructValidator
}

func NewMapValidator(o client.Object) mapValidator {
	return mapValidator{NewDatastructValidator(o)}
}

func ValidateMapSpecCreate(m *Map) error {
	v := NewMapValidator(m)
	v.validateDataStructureSpec(&m.Spec.DataStructureSpec)
	return v.Err()
}

func ValidateMapSpecUpdate(m *Map) error {
	v := NewMapValidator(m)
	v.validateMapSpecUpdate(m)
	v.validateDataStructureSpec(&m.Spec.DataStructureSpec)
	return v.Err()
}

func ValidateMapSpec(m *Map, h *Hazelcast) error {
	v := NewMapValidator(m)
	v.validateMapSpecCurrent(m, h)
	v.validateMapSpecUpdate(m)
	v.validateDataStructureSpec(&m.Spec.DataStructureSpec)
	return v.Err()
}

func (v *mapValidator) validateMapSpecCurrent(m *Map, h *Hazelcast) {
	v.validateMapPersistence(m, h)
	v.validateMapNativeMemory(m, h)
	v.validateNearCacheMemory(m, h)
	v.validateMapTieredStore(m, h)
}

func (v *mapValidator) validateMapPersistence(m *Map, h *Hazelcast) {
	if !m.Spec.PersistenceEnabled {
		return
	}

	lastSpec := v.getHzSpec(h)
	if lastSpec == nil {
		return
	}

	if !lastSpec.Persistence.IsEnabled() {
		v.Invalid(Path("spec", "persistenceEnabled"), lastSpec.Persistence.IsEnabled(), "Persistence must be enabled at Hazelcast")
		return
	}
}

func (v *mapValidator) validateNearCacheMemory(m *Map, h *Hazelcast) {
	if m.Spec.NearCache == nil {
		return
	}

	if m.Spec.NearCache.InMemoryFormat != InMemoryFormatNative {
		return
	}

	lastSpec := v.getHzSpec(h)
	if lastSpec == nil {
		return
	}

	if !lastSpec.NativeMemory.IsEnabled() {
		v.Invalid(Path("spec", "inMemoryFormat"), lastSpec.NativeMemory.IsEnabled(), "Native Memory must be enabled at Hazelcast")
		return
	}
}

func (v *mapValidator) validateMapNativeMemory(m *Map, h *Hazelcast) {
	if m.Spec.InMemoryFormat != InMemoryFormatNative {
		return
	}

	lastSpec := v.getHzSpec(h)
	if lastSpec == nil {
		return
	}

	if !lastSpec.NativeMemory.IsEnabled() {
		v.Invalid(Path("spec", "inMemoryFormat"), lastSpec.NativeMemory.IsEnabled(), "Native Memory must be enabled at Hazelcast")
		return
	}
}

func (v *mapValidator) validateMapTieredStore(m *Map, h *Hazelcast) {
	if m.Spec.TieredStore == nil {
		return
	}
	if m.Spec.PersistenceEnabled {
		v.Invalid(Path("spec", "persistenceEnabled"), m.Spec.PersistenceEnabled, "Tiered store and data persistence are mutually exclusive features. Persistence must be disabled to enable the Tiered Storage")
	}
	if m.Spec.InMemoryFormat != InMemoryFormatNative {
		v.Invalid(Path("spec", "inMemoryFormat"), m.Spec.InMemoryFormat, "In-memory format of the map must be NATIVE to enable the Tiered Storage")
	}
	if len(m.Spec.Indexes) != 0 {
		v.Invalid(Path("spec", "indexes"), m.Spec.Indexes, "Indexes is not supported for Tiered-Store map")
	}
	if !(m.Spec.Eviction.EvictionPolicy == "" || m.Spec.Eviction.EvictionPolicy == EvictionPolicyNone) {
		v.Invalid(Path("spec", "eviction", "evictionPolicy"), m.Spec.Eviction.EvictionPolicy, "Eviction is not supported for Tiered-Store map")
	}
	if m.Spec.TimeToLiveSeconds != 0 {
		v.Invalid(Path("spec", "timeToLiveSeconds"), m.Spec.TimeToLiveSeconds, "TTL expiry is not supported for Tiered-Store map")
	}
	if m.Spec.MaxIdleSeconds != 0 {
		v.Invalid(Path("spec", "maxIdleSeconds"), m.Spec.MaxIdleSeconds, "MaxIdle expiry is not supported for Tiered-Store map")
	}

	lastSpec := v.getHzSpec(h)
	if lastSpec == nil {
		return
	}

	deviceSize := deviceSize(lastSpec.LocalDevices, m.Spec.TieredStore.DiskDeviceName)
	if deviceSize == 0 {
		v.Invalid(Path("spec", "tieredStore", "diskDeviceName"), m.Spec.TieredStore.DiskDeviceName, fmt.Sprintf("device with the name %s does not exist", m.Spec.TieredStore.DiskDeviceName))
	}
	if deviceSize < m.Spec.TieredStore.MemoryCapacity.Value() {
		v.Invalid(Path("spec", "tieredStore", "memoryCapacity"), m.Spec.TieredStore.MemoryCapacity, fmt.Sprintf("Tiered Storage in-memory tier must be smaller than the disk tier. Map memoryCapacity is %v and local device size is %v", m.Spec.TieredStore.MemoryCapacity.Value(), deviceSize))
	}
	v.validateTSMemory(m, h)
}

func deviceSize(localDevices []LocalDeviceConfig, deviceName string) int64 {
	for _, localDevice := range localDevices {
		if localDevice.Name == deviceName {
			return localDevice.PVC.RequestStorage.Value()
		}
	}
	return 0
}

func (v *mapValidator) validateTSMemory(m *Map, h *Hazelcast) {
	mapMemoryCapacity := m.Spec.TieredStore.MemoryCapacity.Value()
	nativeMemorySize := h.Spec.NativeMemory.Size.Value()
	if nativeMemorySize < mapMemoryCapacity {
		v.Invalid(Path("spec", "tieredStore", "memoryCapacity"), m.Spec.TieredStore.MemoryCapacity, fmt.Sprintf("Memory capacity must be less than Native Memory size. Map memoryCapacity is %v and Native Memory size is %v", mapMemoryCapacity, nativeMemorySize))
	}
}

func (v *mapValidator) validateMapSpecUpdate(m *Map) {
	last, ok := m.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return
	}
	var parsed MapSpec
	if err := json.Unmarshal([]byte(last), &parsed); err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last Map spec for update errors: %w", err))
		return
	}

	v.validateNotUpdatableMapFields(&m.Spec, &parsed)
}

func (v *mapValidator) validateNotUpdatableMapFields(current *MapSpec, last *MapSpec) {
	if current.Name != last.Name {
		v.Forbidden(Path("spec", "name"), "field cannot be updated")
	}
	if *current.BackupCount != *last.BackupCount {
		v.Forbidden(Path("spec", "backupCount"), "field cannot be updated")
	}
	if current.AsyncBackupCount != last.AsyncBackupCount {
		v.Forbidden(Path("spec", "asyncBackupCount"), "field cannot be updated")
	}
	if !indexConfigSliceEquals(current.Indexes, last.Indexes) {
		v.Forbidden(Path("spec", "indexes"), "field cannot be updated")
	}
	if current.PersistenceEnabled != last.PersistenceEnabled {
		v.Forbidden(Path("spec", "persistenceEnabled"), "field cannot be updated")
	}
	if current.HazelcastResourceName != last.HazelcastResourceName {
		v.Forbidden(Path("spec", "hazelcastResourceName"), "field cannot be updated")
	}
	if current.InMemoryFormat != last.InMemoryFormat {
		v.Forbidden(Path("spec", "inMemoryFormat"), "field cannot be updated")
	}
	if !reflect.DeepEqual(current.EventJournal, last.EventJournal) {
		v.Forbidden(Path("spec", "eventJournal"), "field cannot be updated")
	}
	if !reflect.DeepEqual(current.NearCache, last.NearCache) {
		v.Forbidden(Path("spec", "nearCache"), "field cannot be updated")
	}
	if !reflect.DeepEqual(current.TieredStore, last.TieredStore) {
		v.Forbidden(Path("spec", "tieredStore"), "field cannot be updated")
	}
}

func indexConfigSliceEquals(a, b []IndexConfig) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if !indexConfigEquals(v, b[i]) {
			return false
		}
	}
	return true
}

func indexConfigEquals(a, b IndexConfig) bool {
	if a.Name != b.Name {
		return false
	}

	if a.Type != b.Type {
		return false
	}

	if !stringSliceEquals(a.Attributes, b.Attributes) {
		return false
	}

	// if both a and b not nil
	if (a.BitmapIndexOptions != nil) && (b.BitmapIndexOptions != nil) {
		return *a.BitmapIndexOptions != *b.BitmapIndexOptions
	}

	// If one of a and b not nil
	if (a.BitmapIndexOptions != nil) || (b.BitmapIndexOptions != nil) {
		return false
	}
	return true
}

func stringSliceEquals(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func (v *mapValidator) getHzSpec(h *Hazelcast) *HazelcastSpec {
	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		v.InternalError(Path("spec"), fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name))
		return nil
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last Hazelcast spec: %w", err))
		return nil
	}
	return lastSpec
}
