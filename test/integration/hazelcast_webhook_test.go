package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/smithy-go/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = Describe("Hazelcast webhook", func() {
	const (
		namespace = "default"
	)
	repository := n.HazelcastRepo
	if ee {
		repository = n.HazelcastEERepo
	}

	defaultSpecValues := &test.HazelcastSpecValues{
		ClusterSize:     n.DefaultClusterSize,
		Repository:      repository,
		Version:         n.HazelcastVersion,
		LicenseKey:      n.LicenseKeySecret,
		ImagePullPolicy: n.HazelcastImagePullPolicy,
	}

	GetRandomObjectMeta := func() metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:      fmt.Sprintf("hazelcast-test-%s", uuid.NewUUID()),
			Namespace: namespace,
		}
	}

	GetRandomObjectMetaWithAnnotation := func(spec *hazelcastv1alpha1.HazelcastSpec) metav1.ObjectMeta {
		hs, _ := json.Marshal(spec)
		return metav1.ObjectMeta{
			Name:      fmt.Sprintf("hazelcast-test-%s", uuid.NewUUID()),
			Namespace: namespace,
			Annotations: map[string]string{
				n.LastSuccessfulSpecAnnotation: string(hs),
			},
		}
	}

	Context("Hazelcast Persistence validation", func() {
		It("should not create HZ PartialStart with FullRecovery", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				BaseDir:                   "/baseDir/",
				ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
				StartupAction:             hazelcastv1alpha1.PartialStart,
				Pvc: hazelcastv1alpha1.PersistencePvcConfiguration{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("startupAction PartialStart can be used only with Partial* clusterDataRecoveryPolicy")))
		})

		It("should not create HZ if pvc is specified", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				BaseDir: "/baseDir/",
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("when persistence is enabled \"pvc\" field must be set")))
		})
	})

	Context("Hazelcast license", func() {
		It("should validate license key for Hazelcast EE", Label("fast"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.LicenseKeySecret = ""

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("when Hazelcast Enterprise is deployed, licenseKeySecret must be set")))
		})
	})

	Context("Hazelcast Cluster Size", func() {
		It(fmt.Sprintf("should validate cluster size is not more than %d", n.ClusterSizeLimit), Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			requestedClusterSize := int32(n.ClusterSizeLimit + 1)
			spec.ClusterSize = &requestedClusterSize

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("cluster size limit is exceeded")))
		})
	})

	Context("JVM Configuration", func() {
		expectedErrStr := `argument %s is configured twice`

		It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.InitialRamPerArg), Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
				Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
					InitialRAMPercentage: ptr.String("10"),
				},
				Args: []string{fmt.Sprintf("%s=10", hazelcastv1alpha1.InitialRamPerArg)},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.InitialRamPerArg))))
		})

		It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.MinRamPerArg), Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
				Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
					MinRAMPercentage: ptr.String("10"),
				},
				Args: []string{fmt.Sprintf("%s=10", hazelcastv1alpha1.MinRamPerArg)},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.MinRamPerArg))))
		})

		It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.MaxRamPerArg), Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
				Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
					MaxRAMPercentage: ptr.String("10"),
				},
				Args: []string{fmt.Sprintf("%s=10", hazelcastv1alpha1.MaxRamPerArg)},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.MaxRamPerArg))))
		})

		It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.GCLoggingArg), Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
				GC: &hazelcastv1alpha1.JVMGCConfiguration{
					Logging: ptr.Bool(true),
				},
				Args: []string{fmt.Sprintf(hazelcastv1alpha1.GCLoggingArg)},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.GCLoggingArg))))
		})

		It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.SerialGCArg), Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			c := hazelcastv1alpha1.GCTypeSerial
			spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
				Memory: nil,
				GC: &hazelcastv1alpha1.JVMGCConfiguration{
					Collector: &c,
				},
				Args: []string{fmt.Sprintf(hazelcastv1alpha1.SerialGCArg)},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.SerialGCArg))))
		})

		It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.ParallelGCArg), Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			c := hazelcastv1alpha1.GCTypeParallel
			spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
				GC:   &hazelcastv1alpha1.JVMGCConfiguration{},
				Args: []string{fmt.Sprintf(hazelcastv1alpha1.ParallelGCArg)},
			}
			spec.JVM.GC.Collector = &c

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.ParallelGCArg))))
		})

		It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.G1GCArg), Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			c := hazelcastv1alpha1.GCTypeG1
			spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
				GC:   &hazelcastv1alpha1.JVMGCConfiguration{},
				Args: []string{fmt.Sprintf(hazelcastv1alpha1.G1GCArg)},
			}
			spec.JVM.GC.Collector = &c

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.G1GCArg))))
		})
	})

	Context("Hazelcast Expose externaly", func() {
		It("should validate MemberAccess for unisocket", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:         hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				MemberAccess: hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("when exposeExternally.type is set to \"Unisocket\", exposeExternally.memberAccess must not be set")))
		})
	})

	Context("Hazelcast HighAvailabilityMode", func() {
		It("should fail to update", Label("fast"), func() {
			zoneHASpec := test.HazelcastSpec(defaultSpecValues, ee)
			zoneHASpec.HighAvailabilityMode = "ZONE"

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMetaWithAnnotation(&zoneHASpec),
				Spec:       zoneHASpec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			test.CheckHazelcastCR(hz, defaultSpecValues, ee)

			var err error
			for {
				Expect(k8sClient.Get(
					context.Background(), types.NamespacedName{Namespace: hz.Namespace, Name: hz.Name}, hz)).Should(Succeed())
				hz.Spec.HighAvailabilityMode = hazelcastv1alpha1.HighAvailabilityNodeMode
				err = k8sClient.Update(context.Background(), hz)
				if errors.IsConflict(err) {
					continue
				}
				break
			}
			Expect(err).
				Should(MatchError(ContainSubstring("highAvailabilityMode cannot be updated")))

			deleteIfExists(lookupKey(hz), hz)
			assertDoesNotExist(lookupKey(hz), hz)
		})
	})

	Context("Hazelcast Advanced Network", func() {
		It("should validate overlap each other", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.AdvancedNetwork = hazelcastv1alpha1.AdvancedNetwork{
				WAN: []hazelcastv1alpha1.WANConfig{
					{
						Port:      5001,
						PortCount: 3,
					},
					{
						Port:      5002,
						PortCount: 3,
					},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(MatchError(
				ContainSubstring("wan replications ports are overlapping, please check and re-apply")))
		})

		It("should validate overlap with other sockets", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.AdvancedNetwork = hazelcastv1alpha1.AdvancedNetwork{
				WAN: []hazelcastv1alpha1.WANConfig{
					{
						Port:      5702,
						PortCount: 3,
					},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("following port numbers are not in use for wan replication")))
		})
	})

})
