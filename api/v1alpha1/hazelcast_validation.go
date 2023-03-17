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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

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
	var allErrs field.ErrorList
	if err := validateExposeExternally(h); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := validateLicense(h); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := validatePersistence(h); err != nil {
		allErrs = append(allErrs, err...)
	}

	if err := validateClusterSize(h); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := validateAdvancedNetwork(h); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Hazelcast"}, h.Name, allErrs)
}

func validateClusterSize(h *Hazelcast) *field.Error {
	if *h.Spec.ClusterSize > naming.ClusterSizeLimit {
		return field.Invalid(field.NewPath("spec").Child("clusterSize"), h.Spec.ClusterSize,
			fmt.Sprintf("may not be greater than %d", naming.ClusterSizeLimit))
	}
	return nil
}

func validateExposeExternally(h *Hazelcast) *field.Error {
	ee := h.Spec.ExposeExternally
	if ee == nil {
		return nil
	}

	if ee.Type == ExposeExternallyTypeUnisocket && ee.MemberAccess != "" {
		return field.Forbidden(field.NewPath("spec").Child("exposeExternally").Child("memberAccess"),
			"can't be set when exposeExternally.type is set to \"Unisocket\"")
	}

	if ee.Type == ExposeExternallyTypeSmart && ee.MemberAccess == MemberAccessNodePortExternalIP {
		if !util.NodeDiscoveryEnabled() {
			return field.Invalid(field.NewPath("spec").Child("exposeExternally").Child("memberAccess"),
				ee.MemberAccess, "value not supported when Hazelcast node discovery is not enabled")
		}
	}

	return nil
}

func validateLicense(h *Hazelcast) *field.Error {
	if checkEnterprise(h.Spec.Repository) && len(h.Spec.LicenseKeySecret) == 0 {
		return field.Required(field.NewPath("spec").Child("licenseKeySecret"),
			"must be set when Hazelcast Enterprise is deployed")
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
			return field.NotFound(field.NewPath("spec").Child("licenseKeySecret"),
				"Hazelcast Enterprise licenseKeySecret is not found")
		}
	}

	return nil
}

func validatePersistence(h *Hazelcast) []*field.Error {
	p := h.Spec.Persistence
	if !p.IsEnabled() {
		return nil
	}
	var allErrs field.ErrorList

	// if hostPath and PVC are both empty or set
	if p.Pvc.IsEmpty() {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("persistence").Child("pvc"),
			"must be set when persistence is enabled"))
	}

	if p.StartupAction == PartialStart && p.ClusterDataRecoveryPolicy == FullRecovery {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("persistence").Child("startupAction"),
			"PartialStart can be used only with Partial* clusterDataRecoveryPolicy"))
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
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

func validateAdvancedNetwork(h *Hazelcast) []*field.Error {
	return validateWANPorts(h)
}

func validateWANPorts(h *Hazelcast) []*field.Error {
	var allErrs field.ErrorList
	err := isOverlapWithEachOther(h)
	if err != nil {
		allErrs = append(allErrs, err)
	}

	err = isOverlapWithOtherSockets(h)
	if err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func isOverlapWithEachOther(h *Hazelcast) *field.Error {
	type portRange struct {
		min uint
		max uint
	}

	var portRanges []portRange
	for _, w := range h.Spec.AdvancedNetwork.WAN {
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
			return field.Duplicate(field.NewPath("spec").Child("advancedNetwork").Child("wan"), fmt.Sprintf("%d", portRanges[i].min))
		}
	}
	return nil
}

func isOverlapWithOtherSockets(h *Hazelcast) *field.Error {
	for _, w := range h.Spec.AdvancedNetwork.WAN {
		min, max := w.Port, w.Port+w.PortCount
		if (n.MemberServerSocketPort >= min && n.MemberServerSocketPort < max) ||
			(n.ClientServerSocketPort >= min && n.ClientServerSocketPort < max) ||
			(n.RestServerSocketPort >= min && n.RestServerSocketPort < max) {
			return field.Duplicate(field.NewPath("spec").Child("advancedNetwork").Child("wan"), fmt.Sprintf("%d", min))
		}
	}
	return nil
}
