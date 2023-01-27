package v1alpha1

import (
	"errors"
	"fmt"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/hazelcast"
	"strings"

	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
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

	if err := validateAdvancedNetwork(h); err != nil {
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

	if p.HostPath != "" && platform.GetType() == platform.OpenShift {
		return errors.New("HostPath persistence is not supported in OpenShift environments")
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

func validateAdvancedNetwork(h *Hazelcast) error {
	if h.Spec.AdvancedNetwork.Enabled {
		for _, w := range h.Spec.AdvancedNetwork.Wan {
			if err := isPortInRange(w.Port, w.PortCount); err != nil {
				return err
			}
		}
	}
	return nil
}

func isPortInRange(port, portCount uint) error {
	if (hazelcast.MemberServerSocketPort >= port && hazelcast.MemberServerSocketPort < port+portCount) ||
		(hazelcast.ClientServerSocketPort >= port && hazelcast.ClientServerSocketPort < port+portCount) ||
		(hazelcast.RestServerSocketPort >= port && hazelcast.RestServerSocketPort < port+portCount) {
		return fmt.Errorf("following port number are not in use for wan replication: %d, %d, %d",
			hazelcast.MemberServerSocketPort,
			hazelcast.ClientServerSocketPort,
			hazelcast.RestServerSocketPort)
	}
	return nil
}
