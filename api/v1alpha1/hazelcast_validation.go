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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

type hazelcastValidator struct {
	fieldValidator
}

func newHazelcastValidator(o client.Object) hazelcastValidator {
	return hazelcastValidator{NewFieldValidator(o)}
}

func ValidateHazelcastSpec(h *Hazelcast) error {
	v := newHazelcastValidator(h)
	v.validateSpecCurrent(h)
	v.validateSpecUpdate(h)
	return v.Err()
}

func (v *hazelcastValidator) validateSpecCurrent(h *Hazelcast) {
	v.validateMetadata(h)
	v.validateExposeExternally(h)
	v.validateLicense(h)
	v.validateTLS(h)
	v.validatePersistence(h)
	v.validateClusterSize(h)
	v.validateAdvancedNetwork(h)
	v.validateJetConfig(h)
	v.validateJVMConfig(h)
	v.validateCustomConfig(h)
	v.validateNativeMemory(h)
	v.validateSQL(h)
	v.validateTieredStorage(h)
	v.validateCPSubsystem(h)
}

func (v *hazelcastValidator) validateSpecUpdate(h *Hazelcast) {
	last, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return
	}
	var parsed HazelcastSpec

	if err := json.Unmarshal([]byte(last), &parsed); err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))
		return
	}

	v.validateNotUpdatableHazelcastFields(&h.Spec, &parsed)

	if h.Spec.CPSubsystem.IsEnabled() && isScaledNotPaused(&h.Spec, &parsed) && !isRevertedToOldSize(h) {
		v.Forbidden(Path("spec", "clusterSize"), "dynamic scaling not permitted when CP is enabled")
	}
}

func (v *hazelcastValidator) validateMetadata(h *Hazelcast) {
	// RFC 1035
	matched, _ := regexp.MatchString(`^[a-zA-Z]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$`, h.Name)
	if !matched {
		v.Invalid(Path("metadata", "name"),
			h.Name, "Hazelcast name has the same constraints as DNS-1035 label."+
				" It must consist of lower case alphanumeric characters or '-',"+
				" start with an alphabetic character, and end with an alphanumeric character"+
				" (e.g. 'my-name',  or 'abc-123', regex used for validation is 'a-z?'")
	}
}

func (v *hazelcastValidator) validateExposeExternally(h *Hazelcast) {
	ee := h.Spec.ExposeExternally
	if ee == nil {
		return
	}

	if ee.Type == ExposeExternallyTypeUnisocket && ee.MemberAccess != "" {
		v.Forbidden(Path("spec", "exposeExternally", "memberAccess"), "can't be set when exposeExternally.type is set to \"Unisocket\"")
	}

	if ee.Type == ExposeExternallyTypeSmart && ee.MemberAccess == MemberAccessNodePortExternalIP {
		if !util.NodeDiscoveryEnabled() {
			v.Invalid(Path("spec", "exposeExternally", "memberAccess"), ee.MemberAccess, "value not supported when Hazelcast node discovery is not enabled")
		}
	}

	supportedTypes := map[corev1.ServiceType]bool{
		corev1.ServiceTypeNodePort:     true,
		corev1.ServiceTypeLoadBalancer: true,
	}

	if ok := supportedTypes[ee.DiscoveryServiceType]; !ok {
		v.Invalid(Path("spec", "exposeExternally", "discoveryServiceType"), ee.DiscoveryServiceType, "service type not supported")
	}
}

func (v *hazelcastValidator) validateCustomConfig(h *Hazelcast) {
	if h.Spec.CustomConfigCmName != "" {
		cmName := types.NamespacedName{
			Name:      h.Spec.CustomConfigCmName,
			Namespace: h.Namespace,
		}
		var cm corev1.ConfigMap
		err := kubeclient.Get(context.Background(), cmName, &cm)
		if kerrors.IsNotFound(err) {
			// we care only about not found error
			v.NotFound(Path("spec", "customConfigCmName"), "ConfigMap for Hazelcast custom configs not found")
		}
	}
}

func (v *hazelcastValidator) validateLicense(h *Hazelcast) {
	if checkEnterprise(h.Spec.Repository) && len(h.Spec.GetLicenseKeySecretName()) == 0 {
		v.Required(Path("spec", "licenseKeySecretName"), "must be set when Hazelcast Enterprise is deployed")
		return
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
			v.NotFound(Path("spec", "licenseKeySecretName"), "Hazelcast Enterprise licenseKeySecret is not found")
			return
		}
	}
}

func (v *hazelcastValidator) validateTLS(h *Hazelcast) {
	// skip validation if TLS is not set
	// deepequal for migration from 5.7 when TLS was not a pointer
	if h.Spec.TLS == nil || reflect.DeepEqual(*h.Spec.TLS, TLS{}) {
		return
	}

	if h.Spec.GetLicenseKeySecretName() == "" {
		v.Required(Path("spec", "tls"), "Hazelcast TLS requires enterprise version")
		return
	}

	p := Path("spec", "tls", "secretName")

	// if user skipped validation secretName can be empty
	if h.Spec.TLS.SecretName == "" {
		v.Required(p, "Hazelcast Enterprise TLS Secret name must be set")
		return
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
		v.NotFound(p, "Hazelcast Enterprise TLS Secret not found")
		return
	}
}

func checkEnterprise(repo string) bool {
	path := strings.Split(repo, "/")
	if len(path) == 0 {
		return false
	}
	return strings.HasSuffix(path[len(path)-1], "-enterprise")
}

func (v *hazelcastValidator) validatePersistence(h *Hazelcast) {
	p := h.Spec.Persistence
	if !p.IsEnabled() {
		return
	}

	if p.PVC == nil {
		v.Required(Path("spec", "persistence", "pvc"), "must be set when persistence is enabled")
	} else {
		if p.PVC.AccessModes == nil {
			v.Required(Path("spec", "persistence", "pvc", "accessModes"), "must be set when persistence is enabled")
		}
	}

	if p.StartupAction == PartialStart && p.ClusterDataRecoveryPolicy == FullRecovery {
		v.Forbidden(Path("spec", "persistence", "startupAction"), "PartialStart can be used only with Partial clusterDataRecoveryPolicy")
	}

	if p.IsRestoreEnabled() && p.Restore.HotBackupResourceName != "" {
		// make sure hot-backup exists
		hbName := types.NamespacedName{
			Name:      p.Restore.HotBackupResourceName,
			Namespace: h.Namespace,
		}

		var hb HotBackup
		err := kubeclient.Get(context.Background(), hbName, &hb)
		if kerrors.IsNotFound(err) {
			// we care only about not found error
			v.NotFound(Path("spec", "persistence", "restore", "hotBackupResourceName"), fmt.Sprintf("There is not hot backup found with name %s", p.Restore.HotBackupResourceName))
		}
	}
}

func (v *hazelcastValidator) validateClusterSize(h *Hazelcast) {
	if *h.Spec.ClusterSize > naming.ClusterSizeLimit {
		v.Invalid(Path("spec", "clusterSize"), h.Spec.ClusterSize, fmt.Sprintf("may not be greater than %d", naming.ClusterSizeLimit))
	}
}

func (v *hazelcastValidator) validateAdvancedNetwork(h *Hazelcast) {
	if h.Spec.AdvancedNetwork == nil {
		return
	}

	v.validateWANServiceTypes(h)
	v.validateWANPorts(h)
}

func (v *hazelcastValidator) validateWANServiceTypes(h *Hazelcast) {
	for i, w := range h.Spec.AdvancedNetwork.WAN {
		if w.ServiceType == corev1.ServiceTypeNodePort || w.ServiceType == corev1.ServiceTypeExternalName {
			v.Forbidden(Path("spec", "advancedNetwork", fmt.Sprintf("wan[%d]", i)), "invalid serviceType value, possible values are ClusterIP and LoadBalancer")
		}
	}

}

func (v *hazelcastValidator) validateWANPorts(h *Hazelcast) {
	v.isOverlapWithEachOther(h)
	v.isOverlapWithOtherSockets(h)
}

func (v *hazelcastValidator) isOverlapWithEachOther(h *Hazelcast) {
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
			v.Invalid(Path("spec", "advancedNetwork", "wan"), fmt.Sprintf("%d-%d", p0.min, p0.max-1), fmt.Sprintf("wan ports overlapping with %d-%d", p1.min, p1.max-1))
		}
	}
}

func (v *hazelcastValidator) isOverlapWithOtherSockets(h *Hazelcast) {
	for i, w := range h.Spec.AdvancedNetwork.WAN {
		min, max := w.Port, w.Port+w.PortCount
		if (n.MemberServerSocketPort >= min && n.MemberServerSocketPort < max) ||
			(n.ClientServerSocketPort >= min && n.ClientServerSocketPort < max) ||
			(n.RestServerSocketPort >= min && n.RestServerSocketPort < max) {
			v.Invalid(Path("spec", "advancedNetwork", fmt.Sprintf("wan[%d]", i)), fmt.Sprintf("%d-%d", min, max-1), fmt.Sprintf("wan ports conflicting with one of %d,%d,%d", n.ClientServerSocketPort, n.MemberServerSocketPort, n.RestServerSocketPort))
		}
	}
}

func (v *hazelcastValidator) validateJVMConfig(h *Hazelcast) {
	jvm := h.Spec.JVM
	if jvm == nil {
		return
	}

	v.validateJVMMemoryArgs(jvm.Memory, jvm.Args)
	v.validateJVMGCArgs(jvm.GC, jvm.Args)
}

func (v *hazelcastValidator) validateJVMMemoryArgs(m *JVMMemoryConfiguration, args []string) {
	if m == nil {
		return
	}

	if m.InitialRAMPercentage != nil {
		v.validateArg(args, InitialRamPerArg)
	}
	if m.MaxRAMPercentage != nil {
		v.validateArg(args, MaxRamPerArg)
	}
	if m.MinRAMPercentage != nil {
		v.validateArg(args, MinRamPerArg)
	}
}

func (v *hazelcastValidator) validateJVMGCArgs(gc *JVMGCConfiguration, args []string) {
	if gc == nil {
		return
	}

	if gc.Logging != nil {
		v.validateArg(args, GCLoggingArg)
	}

	if c := gc.Collector; c != nil {
		if *c == GCTypeSerial {
			v.validateArg(args, SerialGCArg)
		}

		if *c == GCTypeParallel {
			v.validateArg(args, ParallelGCArg)
		}
		if *c == GCTypeG1 {
			v.validateArg(args, G1GCArg)
		}
	}
}

func (v *hazelcastValidator) validateArg(args []string, arg string) {
	for _, s := range args {
		if strings.Contains(s, arg) {
			v.Duplicate(Path("spec", "jvm", "args"), fmt.Sprintf("%s is already set up in JVM config", arg))
			return
		}
	}
}

func (v *hazelcastValidator) validateNotUpdatableHazelcastFields(current *HazelcastSpec, last *HazelcastSpec) {
	if current.HighAvailabilityMode != last.HighAvailabilityMode {
		v.Forbidden(Path("spec", "highAvailabilityMode"), "field cannot be updated")
	}

	v.validateNotUpdatableHzPersistenceFields(current.Persistence, last.Persistence)
	v.validateNotUpdatableSQLFields(current.SQL, last.SQL)
	v.validateNotUpdatableCPFields(current.CPSubsystem, last.CPSubsystem)
}

func (v *hazelcastValidator) validateNotUpdatableHzPersistenceFields(current, last *HazelcastPersistenceConfiguration) {
	if current == nil && last == nil {
		return
	}

	if current == nil && last != nil {
		v.Forbidden(Path("spec", "persistence"), "field cannot be enabled after creation")
		return
	}
	if current != nil && last == nil {
		v.Forbidden(Path("spec", "persistence"), "field cannot be disabled after creation")
		return
	}
	if !reflect.DeepEqual(current.PVC, last.PVC) {
		v.Forbidden(Path("spec", "persistence", "pvc"), "field cannot be updated")
	}
	if !reflect.DeepEqual(current.Restore, last.Restore) {
		v.Forbidden(Path("spec", "persistence", "restore"), "field cannot be updated")
	}
}

func (v *hazelcastValidator) validateNotUpdatableCPFields(current, last *CPSubsystem) {
	if current == nil && last == nil {
		return
	}

	if current == nil && last != nil {
		v.Forbidden(Path("spec", "cpSubsystem"), "field cannot be enabled after creation")
		return
	}
	if current != nil && last == nil {
		v.Forbidden(Path("spec", "cpSubsystem"), "field cannot be disabled after creation")
		return
	}
	if !reflect.DeepEqual(current.PVC, last.PVC) {
		v.Forbidden(Path("spec", "cpSubsystem", "pvc"), "field cannot be updated")
	}
}

func (v *hazelcastValidator) validateNotUpdatableSQLFields(current, last *SQL) {
	if last != nil && last.CatalogPersistenceEnabled && (current == nil || !current.CatalogPersistenceEnabled) {
		v.Forbidden(Path("spec", "sql", "catalogPersistenceEnabled"), "field cannot be disabled after it has been enabled")
	}
}

func (v *hazelcastValidator) validateJetConfig(h *Hazelcast) {
	j := h.Spec.JetEngineConfiguration
	p := h.Spec.Persistence

	if !j.IsEnabled() {
		return
	}

	if j.IsBucketEnabled() {
		if j.BucketConfiguration.GetSecretName() != "" {
			secretName := types.NamespacedName{
				Name:      j.BucketConfiguration.SecretName,
				Namespace: h.Namespace,
			}
			var secret corev1.Secret
			err := kubeclient.Get(context.Background(), secretName, &secret)
			if kerrors.IsNotFound(err) {
				// we care only about not found error
				v.Required(Path("spec", "jet", "bucketConfig", "secretName"), "Bucket credentials Secret not found")
			}
		}
	}

	if j.Instance.IsConfigured() && j.Instance.LosslessRestartEnabled && !p.IsEnabled() {
		v.Forbidden(Path("spec", "jet", "instance", "losslessRestartEnabled"), "can be enabled only if persistence enabled")
	}
}

func (v *hazelcastValidator) validateNativeMemory(h *Hazelcast) {
	// skip validation if NativeMemory is not set
	if h.Spec.NativeMemory == nil {
		return
	}

	if h.Spec.GetLicenseKeySecretName() == "" {
		v.Required(Path("spec", "nativeMemory"), "Hazelcast Native Memory requires enterprise version")
	}

	if h.Spec.Persistence.IsEnabled() && h.Spec.NativeMemory.AllocatorType != NativeMemoryPooled {
		v.Required(Path("spec", "nativeMemory", "allocatorType"), "MemoryAllocatorType.STANDARD cannot be used when Persistence is enabled, Please use MemoryAllocatorType.POOLED!")
	}
}

func (v *hazelcastValidator) validateSQL(h *Hazelcast) {
	// skip validation if SQL is not set
	if h.Spec.SQL == nil {
		return
	}

	if h.Spec.SQL.CatalogPersistenceEnabled && !h.Spec.Persistence.IsEnabled() {
		v.Forbidden(Path("spec", "sql", "catalogPersistence"), "catalogPersistence requires Hazelcast persistence enabled")
	}
}

func (v *hazelcastValidator) validateTieredStorage(h *Hazelcast) {
	// skip validation if LocalDevice is not set
	if !h.Spec.IsTieredStorageEnabled() {
		return
	}
	lds := h.Spec.LocalDevices

	if h.Spec.GetLicenseKeySecretName() == "" {
		v.Required(Path("spec", "localDevices"), "Hazelcast Tiered Storage requires enterprise version")
	}

	if !h.Spec.NativeMemory.IsEnabled() {
		v.Required(Path("spec", "nativeMemory"), "Native Memory must be enabled at Hazelcast when Tiered Storage is enabled")
	}
	for _, ld := range lds {
		v.validateLocalDevice(ld)
	}
}

func (v *hazelcastValidator) validateLocalDevice(ld LocalDeviceConfig) {
	if ld.PVC == nil {
		v.Required(Path("spec", "localDevices", "pvc"), "must be set when LocalDevice is defined")
		return
	}
	if ld.PVC.AccessModes == nil {
		v.Required(Path("spec", "localDevices", "pvc", "accessModes"), "must be set when LocalDevice is defined")
	}
}

func (v *hazelcastValidator) validateCPSubsystem(h *Hazelcast) {
	if h.Spec.CPSubsystem == nil {
		return
	}

	cp := h.Spec.CPSubsystem
	memberSize := pointer.Int32Deref(h.Spec.ClusterSize, 3)
	if memberSize != 0 && memberSize != 3 && memberSize != 5 && memberSize != 7 {
		v.Invalid(Path("spec", "clusterSize"), h.Spec.ClusterSize, "cluster with CP Subsystem enabled can have 3, 5, or 7 members")
	}

	if cp.PVC == nil && (!h.Spec.Persistence.IsEnabled() || h.Spec.Persistence.PVC == nil) {
		v.Required(Path("spec", "cpSubsystem", "pvc"), "PVC should be configured")
	}
	v.validateSessionTTLSeconds(h.Spec.CPSubsystem)
}

func (v *hazelcastValidator) validateSessionTTLSeconds(cp *CPSubsystem) {
	ttl := pointer.Int32Deref(cp.SessionTTLSeconds, 300)
	heartbeat := pointer.Int32Deref(cp.SessionHeartbeatIntervalSeconds, 5)
	autoremoval := pointer.Int32Deref(cp.MissingCpMemberAutoRemovalSeconds, 14400)
	if ttl <= heartbeat {
		v.Invalid(Path("spec", "cpSubsystem", "sessionTTLSeconds"), ttl, "must be greater than sessionHeartbeatIntervalSeconds")
	}
	if autoremoval != 0 && ttl > autoremoval {
		v.Invalid(Path("spec", "cpSubsystem", "sessionTTLSeconds"), ttl, "must be smaller than or equal to missingCpMemberAutoRemovalSeconds")
	}
}

func isScaledNotPaused(current *HazelcastSpec, last *HazelcastSpec) bool {
	newSize := pointer.Int32Deref(current.ClusterSize, 3)
	oldSize := pointer.Int32Deref(last.ClusterSize, 3)
	return newSize != 0 && oldSize != 0 && newSize != oldSize
}

func isRevertedToOldSize(h *Hazelcast) bool {
	newSize := pointer.Int32Deref(h.Spec.ClusterSize, 3)
	return newSize == h.Status.ClusterSize
}
