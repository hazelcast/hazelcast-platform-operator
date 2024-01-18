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
					WanSource: hazelcastv1alpha1.WanSource{
						Config: &hazelcastv1alpha1.WanPublisherConfig{
							Resources: []hazelcastv1alpha1.ResourceSpec{{
								Name: "hazelcast",
								Kind: hazelcastv1alpha1.ResourceKindHZ,
							}},
							TargetClusterName: "dev",
							Endpoints:         "203.0.113.51:5701",
							Queue: hazelcastv1alpha1.QueueSetting{
								Capacity:     10,
								FullBehavior: hazelcastv1alpha1.DiscardAfterMutation,
							},
							Batch: hazelcastv1alpha1.BatchSetting{
								Size:         100,
								MaximumDelay: 1000,
							},
							Acknowledgement: hazelcastv1alpha1.AcknowledgementSetting{
								Type:    hazelcastv1alpha1.AckOnOperationComplete,
								Timeout: 6000,
							},
						},
					},
				}
				wrs, _ := json.Marshal(spec)
				wr := &hazelcastv1alpha1.WanSync{
					ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(wrs)),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), wr)).Should(Succeed())

				Expect(updateCR(wr, func(obj *hazelcastv1alpha1.WanSync) {
					wr.Spec.WanSource.Config.TargetClusterName = "prod"
					wr.Spec.WanSource.Config.Endpoints = "203.0.113.52:5701"
					wr.Spec.WanSource.Config.Queue = hazelcastv1alpha1.QueueSetting{
						Capacity:     20,
						FullBehavior: hazelcastv1alpha1.ThrowException,
					}
					wr.Spec.WanSource.Config.Batch = hazelcastv1alpha1.BatchSetting{
						Size:         200,
						MaximumDelay: 2000,
					}
					wr.Spec.WanSource.Config.Acknowledgement = hazelcastv1alpha1.AcknowledgementSetting{
						Type:    hazelcastv1alpha1.AckOnReceipt,
						Timeout: 10000,
					}
				})).Should(And(
					MatchError(ContainSubstring("spec.config.targetClusterName")),
					MatchError(ContainSubstring("spec.config.endpoints")),
					MatchError(ContainSubstring("spec.config.queue")),
					MatchError(ContainSubstring("spec.config.batch")),
					MatchError(ContainSubstring("spec.config.acknowledgement")),
					MatchError(ContainSubstring("Forbidden: field cannot be updated")),
				))
			})
		})
	})
})
