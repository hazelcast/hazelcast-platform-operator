package integration

import (
	"context"
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
				HostPath:                  "/host/path",
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("startupAction PartialStart can be used only with Partial* clusterDataRecoveryPolicy")))
		})

		It("should not create HZ if none of hostPath and pvc are specified", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				BaseDir: "/baseDir/",
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("when persistence is set either of \"hostPath\" or \"pvc\" fields must be set")))
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
})
