package integration

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("WanReplication CR", func() {
	const namespace = "default"

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.WanReplication{}, &hazelcastv1alpha1.WanReplicationList{}, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.WanSync{}, &hazelcastv1alpha1.WanSyncList{}, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("webhook validation", func() {
		When("updating unmodifiable fields", func() {
			It("should not be allowed", Label("fast"), func() {
				spec := hazelcastv1alpha1.WanSyncSpec{
					WanReplicationName: "existing-wan-replication",
				}
				wrs, _ := json.Marshal(spec)
				wr := &hazelcastv1alpha1.WanSync{
					ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(wrs)),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), wr)).Should(Succeed())

				Expect(updateCR(wr, func(obj *hazelcastv1alpha1.WanSync) {
					wr.Spec.WanReplicationName = "new-wan-replication"
				})).Should(
					MatchError(ContainSubstring("spec.wanReplicationName")),
				)
			})
		})
	})
})