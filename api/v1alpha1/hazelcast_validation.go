package v1alpha1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/hazelcast/hazelcast-platform-operator/internal/kubeclient"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

const (
	MemberServerSocketPort = 5702
	ClientServerSocketPort = 5701
	RestServerSocketPort   = 8080
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
			return errors.New("secret not found")
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
	for _, w := range h.Spec.AdvancedNetwork.Wan {
		if err := isPortInRange(w.Port, w.PortCount); err != nil {
			return err
		}
	}
	return nil
}

func isPortInRange(port, portCount uint) error {
	//TODO: check if there is overlapping port numbers between wan replication configurations
	if (MemberServerSocketPort >= port && MemberServerSocketPort < port+portCount) ||
		(ClientServerSocketPort >= port && ClientServerSocketPort < port+portCount) ||
		(RestServerSocketPort >= port && RestServerSocketPort < port+portCount) {
		return fmt.Errorf("following port number are not in use for wan replication: %d, %d, %d",
			MemberServerSocketPort,
			ClientServerSocketPort,
			RestServerSocketPort)
	}
	return nil
}
