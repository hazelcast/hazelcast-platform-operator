package integration

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

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
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("with default configuration", func() {
		It("should create successfully", func() {
			wan := &hazelcastv1alpha1.WanReplication{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.WanReplicationSpec{
					Resources: []hazelcastv1alpha1.ResourceSpec{
						{
							Name: "hazelcast",
							Kind: hazelcastv1alpha1.ResourceKindHZ,
						},
					},
					TargetClusterName: "dev",
					Endpoints:         "10.0.0.1:5701",
				},
			}
			By("creating WanReplication CR successfully")
			Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())
			ws := wan.Spec

			By("checking the CR values with default ones")
			Expect(ws.Resources).To(HaveLen(1))
			Expect(ws.Resources[0].Kind).To(Equal(hazelcastv1alpha1.ResourceKindHZ))
			Expect(ws.Resources[0].Name).To(Equal("hazelcast"))
			Expect(ws.TargetClusterName).To(Equal("dev"))
			Expect(ws.Endpoints).To(Equal("10.0.0.1:5701"))
		})

		When("applying empty spec", func() {
			It("should fail to create", func() {
				wan := &hazelcastv1alpha1.WanReplication{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("trying to create WAN CR")
				Expect(k8sClient.Create(context.Background(), wan)).ShouldNot(Succeed())
			})
		})
	})

	Context("with Endpoints value", func() {
		When("endpoints are configured without port", func() {
			It("should set default port to endpoints", func() {
				endpoints := "10.0.0.1,10.0.0.2,10.0.0.3"
				wan := &hazelcastv1alpha1.WanReplication{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.WanReplicationSpec{
						Resources: []hazelcastv1alpha1.ResourceSpec{
							{
								Name: "hazelcast",
								Kind: hazelcastv1alpha1.ResourceKindHZ,
							},
						},
						TargetClusterName: "dev",
						Endpoints:         endpoints,
					},
				}
				By("creating WanReplication CR successfully")
				Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())
				Eventually(func() string {
					newWan := hazelcastv1alpha1.WanReplication{}
					err := k8sClient.Get(context.Background(), types.NamespacedName{Name: wan.Name, Namespace: wan.Namespace}, &newWan)
					if err != nil {
						return ""
					}
					return newWan.Spec.Endpoints
				}, timeout, interval).Should(Equal("10.0.0.1:5710,10.0.0.2:5710,10.0.0.3:5710"))
			})
		})
	})

	Context("webhook validation", func() {
		When("updating unmodifiable fields", func() {
			It("should not be allowed", func() {
				spec := hazelcastv1alpha1.WanReplicationSpec{
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
				}
				wrs, _ := json.Marshal(spec)
				wr := &hazelcastv1alpha1.WanReplication{
					ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(wrs)),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), wr)).Should(Succeed())

				Expect(updateCR(wr, func(obj *hazelcastv1alpha1.WanReplication) {
					wr.Spec.TargetClusterName = "prod"
					wr.Spec.Endpoints = "203.0.113.52:5701"
					wr.Spec.Queue = hazelcastv1alpha1.QueueSetting{
						Capacity:     20,
						FullBehavior: hazelcastv1alpha1.ThrowException,
					}
					wr.Spec.Batch = hazelcastv1alpha1.BatchSetting{
						Size:         200,
						MaximumDelay: 2000,
					}
					wr.Spec.Acknowledgement = hazelcastv1alpha1.AcknowledgementSetting{
						Type:    hazelcastv1alpha1.AckOnReceipt,
						Timeout: 10000,
					}
				})).Should(And(
					MatchError(ContainSubstring("spec.targetClusterName")),
					MatchError(ContainSubstring("spec.endpoints")),
					MatchError(ContainSubstring("spec.queue")),
					MatchError(ContainSubstring("spec.batch")),
					MatchError(ContainSubstring("spec.acknowledgement")),
					MatchError(ContainSubstring("Forbidden: field cannot be updated")),
				))
			})
		})
	})
})
