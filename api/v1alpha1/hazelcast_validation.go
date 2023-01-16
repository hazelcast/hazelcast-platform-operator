package v1alpha1

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var BlackListProperties = map[string]struct{}{
	// TODO: Add properties which should not be exposed.
	"": {},
}

func ValidateNotUpdatableHazelcastFields(current *HazelcastSpec, last *HazelcastSpec) error {
	if current.HighAvailabilityMode != last.HighAvailabilityMode {
		return errors.New("highAvailabilityMode cannot be updated")
	}
	return nil
}

func ValidateHazelcastSpec(h *Hazelcast) error {
	if err := validateExposeExternally(h); err != nil {
		return err
	}

	if err := validateLicense(h); err != nil {
		return err
	}

	if err := validatePersistence(h); err != nil {
		return err
	}

	if err := validateClusterSize(h); err != nil {
		return err
	}

	if err := validateNativeMemory(h); err != nil {
		return err
	}

	return nil
}

func validateClusterSize(h *Hazelcast) error {
	if *h.Spec.ClusterSize > naming.ClusterSizeLimit {
		return fmt.Errorf("cluster size limit is exceeded. Requested: %d, Limit: %d", *h.Spec.ClusterSize, naming.ClusterSizeLimit)
	}
	return nil
}

func validateExposeExternally(h *Hazelcast) error {
	ee := h.Spec.ExposeExternally
	if ee == nil {
		return nil
	}

	if ee.Type == ExposeExternallyTypeUnisocket && ee.MemberAccess != "" {
		return errors.New("when exposeExternally.type is set to \"Unisocket\", exposeExternally.memberAccess must not be set")
	}

	return nil
}

func validateLicense(h *Hazelcast) error {
	if checkEnterprise(h.Spec.Repository) && len(h.Spec.LicenseKeySecret) == 0 {
		return errors.New("when Hazelcast Enterprise is deployed, licenseKeySecret must be set")
	}
	return nil
}

func validatePersistence(h *Hazelcast) error {
	p := h.Spec.Persistence
	if !p.IsEnabled() {
		return nil
	}

	// if hostPath and PVC are both empty or set
	if (p.HostPath == "") == p.Pvc.IsEmpty() {
		return errors.New("when persistence is set either of \"hostPath\" or \"pvc\" fields must be set")
	}

	if p.StartupAction == PartialStart && p.ClusterDataRecoveryPolicy == FullRecovery {
		return errors.New("startupAction PartialStart can be used only with Partial* clusterDataRecoveryPolicy")
	}
	return nil
}

func checkEnterprise(repo string) bool {
	path := strings.Split(repo, "/")
	if len(path) == 0 {
		return false
	}
	return strings.HasSuffix(path[len(path)-1], "-enterprise")
}

func validateNativeMemory(h *Hazelcast) error {
	if !h.Spec.NativeMemory.GetEnabled() {
		// skip validation
		return nil
	}

	if h.Spec.NativeMemory.GetAllocatorType() == NativeMemoryPooled {
		// pooled supports all options, skip validation
		return nil
	}

	if h.Spec.NativeMemory.MinBlockSize != nil {
		return errors.New("native-memory: MinBlockSize is used only by the POOLED memory allocator")
	}

	if h.Spec.NativeMemory.PageSize != nil {
		return errors.New("native-memory: PageSize is used only by the POOLED memory allocator")
	}

	if h.Spec.NativeMemory.MetadataSpacePercentage != nil {
		return errors.New("native-memory: MetadataSpacePercentage is used only by the POOLED memory allocator")
	}

	return nil
}
