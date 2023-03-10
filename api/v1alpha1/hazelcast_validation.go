package v1alpha1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/hazelcast/hazelcast-platform-operator/internal/kubeclient"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
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

	if ee.Type == ExposeExternallyTypeSmart && ee.MemberAccess == MemberAccessNodePortExternalIP {
		if !util.NodeDiscoveryEnabled() {
			return errors.New("when Hazelcast node discovery is not enabled, exposeExternally.MemberAccess cannot be set to `NodePortExternalIP`")
		}
	}

	return nil
}

func validateLicense(h *Hazelcast) error {
	if checkEnterprise(h.Spec.Repository) && len(h.Spec.LicenseKeySecret) == 0 {
		return errors.New("when Hazelcast Enterprise is deployed, licenseKeySecret must be set")
	}

	// make sure secret exists
	if h.Spec.LicenseKeySecret != "" {
		secretName := types.NamespacedName{
			Name:      h.Spec.LicenseKeySecret,
			Namespace: h.Namespace,
		}

		var secret corev1.Secret
		err := kubeclient.Get(context.Background(), secretName, &secret)
		if kerrors.IsNotFound(err) {
			// we care only about not found error
			return errors.New("Hazelcast Enterprise licenseKeySecret is not found")
		}
	}

	return nil
}

func validatePersistence(h *Hazelcast) error {
	p := h.Spec.Persistence
	if !p.IsEnabled() {
		return nil
	}

	// if hostPath and PVC are both empty or set
	if p.Pvc.IsEmpty() {
		return errors.New("when persistence is enabled \"pvc\" field must be set")
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

func ValidateAppliedPersistence(persistenceEnabled bool, h *Hazelcast) error {
	if !persistenceEnabled {
		return nil
	}
	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name)
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		return fmt.Errorf("last successful spec for Hazelcast resource %s is not formatted correctly", h.Name)
	}

	if !lastSpec.Persistence.IsEnabled() {
		return fmt.Errorf("persistence is not enabled for the Hazelcast resource %s", h.Name)
	}

	return nil
}

func validateAdvancedNetwork(h *Hazelcast) error {
	if h.Spec.AdvancedNetwork == nil {
		return nil
	}

	return validateWANPorts(h)
}

func validateWANPorts(h *Hazelcast) error {
	err := isOverlapWithEachOther(h)
	if err != nil {
		return err
	}

	err = isOverlapWithOtherSockets(h)
	if err != nil {
		return err
	}
	return nil
}

func isOverlapWithEachOther(h *Hazelcast) error {
	type portRange struct {
		min uint
		max uint
	}

	var portRanges []portRange
	for _, w := range h.Spec.AdvancedNetwork.Wan {
		portRanges = append(portRanges, struct {
			min uint
			max uint
		}{min: w.Port, max: w.Port + w.PortCount})
	}

	sort.Slice(portRanges, func(r1, r2 int) bool {
		return portRanges[r1].min < portRanges[r2].min
	})

	for i := 1; i < len(portRanges); i++ {
		if portRanges[i-1].max > portRanges[i].min {
			return fmt.Errorf("wan replications ports are overlapping, please check and re-apply")
		}
	}
	return nil
}

func isOverlapWithOtherSockets(h *Hazelcast) error {
	for _, w := range h.Spec.AdvancedNetwork.Wan {
		min, max := w.Port, w.Port+w.PortCount
		if (n.MemberServerSocketPort >= min && n.MemberServerSocketPort < max) ||
			(n.ClientServerSocketPort >= min && n.ClientServerSocketPort < max) ||
			(n.RestServerSocketPort >= min && n.RestServerSocketPort < max) {
			return fmt.Errorf("following port numbers are not in use for wan replication: %d, %d, %d",
				n.MemberServerSocketPort,
				n.ClientServerSocketPort,
				n.RestServerSocketPort)
		}
	}
	return nil
}
