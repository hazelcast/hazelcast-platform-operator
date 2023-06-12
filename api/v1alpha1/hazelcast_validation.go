package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
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

const (
	InitialRamPerArg = "-XX:InitialRAMPercentage"
	MaxRamPerArg     = "-XX:MaxRAMPercentage"
	MinRamPerArg     = "-XX:MinRAMPercentage"
	GCLoggingArg     = "-verbose:gc"
	SerialGCArg      = "-XX:+UseSerialGC"
	ParallelGCArg    = "-XX:+UseParallelGC"
	G1GCArg          = "-XX:+UseG1GC"
)

var BlackListProperties = map[string]struct{}{
	// TODO: Add properties which should not be exposed.
	"": {},
}

func ValidateHazelcastSpec(h *Hazelcast) error {
	currentErrs := ValidateHazelcastSpecCurrent(h)
	updateErrs := ValidateHazelcastSpecUpdate(h)
	allErrs := append(currentErrs, updateErrs...)
	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Hazelcast"}, h.Name, allErrs)
}

func ValidateHazelcastSpecCurrent(h *Hazelcast) []*field.Error {
	var allErrs field.ErrorList

	if err := validateMetadata(h); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := validateExposeExternally(h); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := validateLicense(h); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := validateTLS(h); err != nil {
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

	if err := validateJetConfig(h); err != nil {
		allErrs = append(allErrs, err...)
	}

	if err := validateJVMConfig(h); err != nil {
		allErrs = append(allErrs, err...)
	}

	if err := validateCustomConfig(h); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := validateNativeMemory(h); err != nil {
		allErrs = append(allErrs, err...)
	}

	return allErrs
}

func validateMetadata(h *Hazelcast) *field.Error {
	// RFC 1035
	matched, _ := regexp.MatchString(`^[a-zA-Z]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$`, h.Name)
	if !matched {
		return field.Invalid(field.NewPath("metadata").Child("name"),
			h.Name, "Hazelcast name has the same constraints as DNS-1035 label."+
				" It must consist of lower case alphanumeric characters or '-',"+
				" start with an alphabetic character, and end with an alphanumeric character"+
				" (e.g. 'my-name',  or 'abc-123', regex used for validation is 'a-z?'")
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

func validateCustomConfig(h *Hazelcast) *field.Error {
	if h.Spec.CustomConfigCmName != "" {
		cmName := types.NamespacedName{
			Name:      h.Spec.CustomConfigCmName,
			Namespace: h.Namespace,
		}
		var cm corev1.ConfigMap
		err := kubeclient.Get(context.Background(), cmName, &cm)
		if kerrors.IsNotFound(err) {
			// we care only about not found error
			return field.NotFound(field.NewPath("spec").Child("customConfigCmName"),
				"ConfigMap for Hazelcast custom configs not found")
		}
	}
	return nil
}

func validateLicense(h *Hazelcast) *field.Error {
	if checkEnterprise(h.Spec.Repository) && len(h.Spec.GetLicenseKeySecretName()) == 0 {
		return field.Required(field.NewPath("spec").Child("licenseKeySecretName"),
			"must be set when Hazelcast Enterprise is deployed")
	}

	// make sure secret exists
	if h.Spec.GetLicenseKeySecretName() != "" {
		secretName := types.NamespacedName{
			Name:      h.Spec.GetLicenseKeySecretName(),
			Namespace: h.Namespace,
		}

		var secret corev1.Secret
		err := kubeclient.Get(context.Background(), secretName, &secret)
		if kerrors.IsNotFound(err) {
			// we care only about not found error
			return field.NotFound(field.NewPath("spec").Child("licenseKeySecretName"),
				"Hazelcast Enterprise licenseKeySecret is not found")
		}
	}

	return nil
}

func validateTLS(h *Hazelcast) *field.Error {
	// skip validation if TLS is not set
	// deepequal for migration from 5.7 when TLS was not a pointer
	if h.Spec.TLS == nil || reflect.DeepEqual(*h.Spec.TLS, TLS{}) {
		return nil
	}

	if h.Spec.GetLicenseKeySecretName() == "" {
		return field.Required(field.NewPath("spec").Child("tls"),
			"Hazelcast TLS requires enterprise version")
	}

	p := field.NewPath("spec").Child("tls").Child("secretName")

	// if user skipped validation secretName can be empty
	if h.Spec.TLS.SecretName == "" {
		return field.Required(p, "Hazelcast Enterprise TLS Secret name must be set")
	}

	// check if secret exists
	secretName := types.NamespacedName{
		Name:      h.Spec.TLS.SecretName,
		Namespace: h.Namespace,
	}

	var secret corev1.Secret
	err := kubeclient.Get(context.Background(), secretName, &secret)
	if kerrors.IsNotFound(err) {
		// we care only about not found error
		return field.NotFound(p, "Hazelcast Enterprise TLS Secret not found")
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
			"PartialStart can be used only with Partial clusterDataRecoveryPolicy"))
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func validateClusterSize(h *Hazelcast) *field.Error {
	if *h.Spec.ClusterSize > naming.ClusterSizeLimit {
		return field.Invalid(field.NewPath("spec").Child("clusterSize"), h.Spec.ClusterSize,
			fmt.Sprintf("may not be greater than %d", naming.ClusterSizeLimit))
	}
	return nil
}

func validateAdvancedNetwork(h *Hazelcast) []*field.Error {
	var allErrs field.ErrorList
	if h.Spec.AdvancedNetwork == nil {
		return allErrs
	}

	if errs := validateWANServiceTypes(h); errs != nil {
		allErrs = append(allErrs, errs...)
	}

	if errs := validateWANPorts(h); errs != nil {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func validateWANServiceTypes(h *Hazelcast) []*field.Error {
	var allErrs field.ErrorList

	for i, w := range h.Spec.AdvancedNetwork.WAN {
		if w.ServiceType == corev1.ServiceTypeNodePort || w.ServiceType == corev1.ServiceTypeExternalName {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("advancedNetwork").Child(fmt.Sprintf("wan[%d]", i)),
				"invalid serviceType value, possible values are ClusterIP and LoadBalancer"))
		}
	}
	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func validateWANPorts(h *Hazelcast) []*field.Error {
	var allErrs field.ErrorList

	if errs := isOverlapWithEachOther(h); errs != nil {
		allErrs = append(allErrs, errs...)
	}
	if errs := isOverlapWithOtherSockets(h); errs != nil {
		allErrs = append(allErrs, errs...)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func isOverlapWithEachOther(h *Hazelcast) []*field.Error {
	var allErrs field.ErrorList

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
		p0 := portRanges[i-1]
		p1 := portRanges[i]
		if p0.max > p1.min {
			err := field.Invalid(field.NewPath("spec").Child("advancedNetwork").Child("wan"),
				fmt.Sprintf("%d-%d", p0.min, p0.max-1),
				fmt.Sprintf("wan ports overlapping with %d-%d", p1.min, p1.max-1))
			allErrs = append(allErrs, err)

		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func isOverlapWithOtherSockets(h *Hazelcast) []*field.Error {
	var allErrs field.ErrorList

	for i, w := range h.Spec.AdvancedNetwork.WAN {
		min, max := w.Port, w.Port+w.PortCount
		if (n.MemberServerSocketPort >= min && n.MemberServerSocketPort < max) ||
			(n.ClientServerSocketPort >= min && n.ClientServerSocketPort < max) ||
			(n.RestServerSocketPort >= min && n.RestServerSocketPort < max) {
			err := field.Invalid(field.NewPath("spec").Child("advancedNetwork").Child(fmt.Sprintf("wan[%d]", i)),
				fmt.Sprintf("%d-%d", min, max-1),
				fmt.Sprintf("wan ports conflicting with one of %d,%d,%d", n.ClientServerSocketPort, n.MemberServerSocketPort, n.RestServerSocketPort))
			allErrs = append(allErrs, err)
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func validateJVMConfig(h *Hazelcast) []*field.Error {
	jvm := h.Spec.JVM
	if jvm == nil {
		return nil
	}
	var allErrs field.ErrorList
	args := jvm.Args

	if err := validateJVMMemoryArgs(jvm.Memory, args); err != nil {
		allErrs = append(allErrs, err...)
	}

	if err := validateJVMGCArgs(jvm.GC, args); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func validateJVMMemoryArgs(m *JVMMemoryConfiguration, args []string) []*field.Error {
	if m == nil {
		return nil
	}
	var allErrs field.ErrorList

	if m.InitialRAMPercentage != nil {
		allErrs = appendIfNotNil(allErrs, validateArg(args, InitialRamPerArg))
	}
	if m.MaxRAMPercentage != nil {
		allErrs = appendIfNotNil(allErrs, validateArg(args, MaxRamPerArg))
	}
	if m.MinRAMPercentage != nil {
		allErrs = appendIfNotNil(allErrs, validateArg(args, MinRamPerArg))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return allErrs
}

func validateJVMGCArgs(gc *JVMGCConfiguration, args []string) []*field.Error {
	if gc == nil {
		return nil
	}

	var allErrs field.ErrorList
	if gc.Logging != nil {
		allErrs = appendIfNotNil(allErrs, validateArg(args, GCLoggingArg))
	}

	if c := gc.Collector; c != nil {
		if *c == GCTypeSerial {
			allErrs = appendIfNotNil(allErrs, validateArg(args, SerialGCArg))
		}

		if *c == GCTypeParallel {
			allErrs = appendIfNotNil(allErrs, validateArg(args, ParallelGCArg))
		}
		if *c == GCTypeG1 {
			allErrs = appendIfNotNil(allErrs, validateArg(args, G1GCArg))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return allErrs
}

func validateArg(args []string, arg string) *field.Error {
	for _, s := range args {
		if strings.Contains(s, arg) {
			return field.Duplicate(field.NewPath("spec").Child("jvm").Child("args"), fmt.Sprintf("%s is already set up in JVM config", arg))
		}
	}
	return nil
}

func ValidateHazelcastSpecUpdate(h *Hazelcast) []*field.Error {
	last, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return nil
	}
	var parsed HazelcastSpec

	if err := json.Unmarshal([]byte(last), &parsed); err != nil {
		return []*field.Error{field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))}
	}

	return ValidateNotUpdatableHazelcastFields(&h.Spec, &parsed)

}

func ValidateNotUpdatableHazelcastFields(current *HazelcastSpec, last *HazelcastSpec) []*field.Error {
	var allErrs field.ErrorList

	if current.HighAvailabilityMode != last.HighAvailabilityMode {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("highAvailabilityMode"), "field cannot be updated"))
	}

	if errs := ValidateNotUpdatableHzPersistenceFields(current.Persistence, last.Persistence); errs != nil {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) == 0 {
		return nil
	}

	return allErrs
}

func ValidateNotUpdatableHzPersistenceFields(current, last *HazelcastPersistenceConfiguration) []*field.Error {
	var allErrs field.ErrorList

	if current == nil && last == nil {
		return nil
	}

	if current == nil && last != nil {
		return append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("persistence"), "field cannot be enabled after creation"))
	}

	if current != nil && last == nil {
		return append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("persistence"), "field cannot be disabled after creation"))
	}

	if current.BaseDir != last.BaseDir {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("persistence").Child("baseDir"), "field cannot be updated"))
	}
	if !reflect.DeepEqual(current.Pvc, last.Pvc) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("persistence").Child("pvc"), "field cannot be updated"))
	}
	if current.Restore != last.Restore {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("persistence").Child("restore"), "field cannot be updated"))
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func validateJetConfig(h *Hazelcast) (errs field.ErrorList) {
	j := h.Spec.JetEngineConfiguration
	p := h.Spec.Persistence

	if !j.IsEnabled() {
		return
	}

	if j.IsBucketEnabled() {
		if j.BucketConfiguration.GetSecretName() == "" {
			errs = append(errs, field.Required(
				field.NewPath("spec").Child("jet").Child("bucketConfig").Child("secretName"),
				"bucket secret must be set"))
		} else {
			secretName := types.NamespacedName{
				Name:      j.BucketConfiguration.SecretName,
				Namespace: h.Namespace,
			}
			var secret corev1.Secret
			err := kubeclient.Get(context.Background(), secretName, &secret)
			if kerrors.IsNotFound(err) {
				// we care only about not found error
				errs = append(errs, field.Required(field.NewPath("spec").Child("jet").Child("bucketConfig").Child("secretName"),
					"Bucket credentials Secret not found"))
			}
		}
	}

	if j.Instance.IsConfigured() && j.Instance.LosslessRestartEnabled && !p.IsEnabled() {
		errs = append(errs,
			field.Forbidden(field.NewPath("spec").Child("jet").Child("instance").Child("losslessRestartEnabled"),
				"can be enabled only if persistence enabled"))
	}

	return
}

func validateNativeMemory(h *Hazelcast) []*field.Error {
	// skip validation if NativeMemory is not set
	if h.Spec.NativeMemory == nil {
		return nil
	}
	var allErrs field.ErrorList
	if h.Spec.GetLicenseKeySecretName() == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("nativeMemory"),
			"Hazelcast Native Memory requires enterprise version"))
	}

	if h.Spec.Persistence.IsEnabled() && h.Spec.NativeMemory.AllocatorType != NativeMemoryPooled {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("nativeMemory").Child("allocatorType"),
			"MemoryAllocatorType.STANDARD cannot be used when Persistence is enabled, Please use MemoryAllocatorType.POOLED!"))
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}
