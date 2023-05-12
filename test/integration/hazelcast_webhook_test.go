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
	"k8s.io/utils/pointer"

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

	BeforeEach(func() {
		if !ee {
			return
		}

		licenseSec := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n.LicenseKeySecret,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				n.LicenseDataKey: []byte("integration-test-license"),
			},
		}

		By(fmt.Sprintf("creating license key secret '%s'", licenseSec.Name))
		Eventually(func() bool {
			err := k8sClient.Create(context.Background(), &licenseSec)
			return err == nil || errors.IsAlreadyExists(err)
		}, timeout, interval).Should(BeTrue())
	})

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
				Should(MatchError(ContainSubstring("PartialStart can be used only with Partial clusterDataRecoveryPolicy")))
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
				Should(MatchError(ContainSubstring("spec.persistence.pvc: Required value: must be set when persistence is enabled")))
		})
	})

	Context("Hazelcast license", func() {
		It("should validate license key for Hazelcast EE", Label("fast"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.LicenseKeySecretName = ""

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("spec.licenseKeySecret: Required value: must be set when Hazelcast Enterprise is deployed")))
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
				Should(MatchError(ContainSubstring("Invalid value: 301: may not be greater than 300")))
		})
	})

	Context("JVM Configuration", func() {
		expectedErrStr := `%s is already set up in JVM config"`

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
				Should(MatchError(ContainSubstring("Forbidden: can't be set when exposeExternally.type is set to \"Unisocket\"")))
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
				Should(MatchError(ContainSubstring("spec.highAvailabilityMode: Forbidden: field cannot be updated")))

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
				ContainSubstring("spec.advancedNetwork.wan: Invalid value: \"5001-5003\": wan ports overlapping with 5002-5004")))
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
				Should(MatchError(ContainSubstring("spec.advancedNetwork.wan[0]: Invalid value: \"5702-5704\": wan ports conflicting with one of 5701,5702,8081")))
		})

		It("should validate service type for wan config", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.AdvancedNetwork = hazelcastv1alpha1.AdvancedNetwork{
				WAN: []hazelcastv1alpha1.WANConfig{
					{
						Port:        5702,
						PortCount:   3,
						ServiceType: corev1.ServiceTypeExternalName,
					},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("invalid serviceType value, possible values are ClusterIP and LoadBalancer")))
		})
	})

	Context("Hazelcast Validation Multiple Errors", func() {
		It("should return multiple errors", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				MemberAccess:         hazelcastv1alpha1.MemberAccessLoadBalancer,
			}
			spec.ClusterSize = pointer.Int32(5000)
			spec.AdvancedNetwork = hazelcastv1alpha1.AdvancedNetwork{
				WAN: []hazelcastv1alpha1.WANConfig{
					{
						Port:      5701,
						PortCount: 20,
					},
					{
						Port:      5709,
						PortCount: 1,
					},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}
			err := k8sClient.Create(context.Background(), hz)
			Expect(err).Should(MatchError(
				ContainSubstring("spec.exposeExternally.memberAccess:")))
			Expect(err).Should(MatchError(
				ContainSubstring("spec.clusterSize:")))
			Expect(err).Should(MatchError(
				ContainSubstring("spec.advancedNetwork.wan:")))
			Expect(err).Should(MatchError(
				ContainSubstring("spec.advancedNetwork.wan[0]:")))

		})

	})
	Context("Hazelcast Jet Engine Configuration", func() {
		It("should validate backup count", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.JetEngineConfiguration = hazelcastv1alpha1.JetEngineConfiguration{
				Enabled: pointer.Bool(true),
				Instance: &hazelcastv1alpha1.JetInstance{
					BackupCount: 7,
				},
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(MatchError(
				ContainSubstring("Invalid value: 7: spec.jet.instance.backupCount in body should be less than or equal to 6")))
		})

		It("should validate if lossless restart enabled without enabling persistence", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.JetEngineConfiguration = hazelcastv1alpha1.JetEngineConfiguration{
				Enabled: pointer.Bool(true),
				Instance: &hazelcastv1alpha1.JetInstance{
					LosslessRestartEnabled: true,
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("Forbidden: can be enabled only if persistence enabled")))
		})
	})
})
