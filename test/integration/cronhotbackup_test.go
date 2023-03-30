package integration

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("CronHotBackup CR", func() {
	const namespace = "default"

	Context("with default configuration", func() {
		It("should create successfully", Label("fast"), func() {
			chb := &hazelcastv1alpha1.CronHotBackup{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.CronHotBackupSpec{
					Schedule: "* * * * *",
					HotBackupTemplate: hazelcastv1alpha1.HotBackupTemplateSpec{
						Spec: hazelcastv1alpha1.HotBackupSpec{},
					},
				},
			}
			By("creating CronHotBackup CR successfully")
			Expect(k8sClient.Create(context.Background(), chb)).Should(Succeed())
			chbs := chb.Spec

			By("checking the CR values with default ones")
			Expect(*chbs.SuccessfulHotBackupsHistoryLimit).To(Equal(n.DefaultSuccessfulHotBackupsHistoryLimit))
			Expect(*chbs.FailedHotBackupsHistoryLimit).To(Equal(n.DefaultFailedHotBackupsHistoryLimit))
			deleteResource(lookupKey(chb), chb)
		})

		When("applying empty spec", func() {
			It("should fail to create", Label("fast"), func() {
				chb := &hazelcastv1alpha1.CronHotBackup{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create CronHotBackup CR")
				Expect(k8sClient.Create(context.Background(), chb)).ShouldNot(Succeed())
				assertDoesNotExist(lookupKey(chb), chb)
			})
		})

		When("giving labels and annotations to HotBackup Template", func() {
			It("should create successfully with those labels", Label("fast"), func() {
				ans := map[string]string{
					"annotation1": "val",
					"annotation2": "val2",
				}
				labels := map[string]string{
					"label1": "val",
					"label2": "val2",
				}
				chb := &hazelcastv1alpha1.CronHotBackup{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.CronHotBackupSpec{
						Schedule: "*/1 * * * * *",
						HotBackupTemplate: hazelcastv1alpha1.HotBackupTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: ans,
								Labels:      labels,
							},
							Spec: hazelcastv1alpha1.HotBackupSpec{},
						},
					},
				}
				By("creating CronHotBackup CR successfully")
				Expect(k8sClient.Create(context.Background(), chb)).Should(Succeed())

				Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: chb.Name, Namespace: chb.Namespace}, chb)).Should(Succeed())
				// Wait for at least two HotBackups to get created
				time.Sleep(2 * time.Second)
				hbl := &hazelcastv1alpha1.HotBackupList{}
				Eventually(func() string {
					err := k8sClient.List(context.Background(), hbl, client.InNamespace(namespace), client.MatchingLabels(labels))
					if err != nil {
						return ""
					}
					if len(hbl.Items) < 1 {
						return ""
					}
					return hbl.Items[0].Name
				}, timeout, interval).Should(ContainSubstring(chb.Name))

				Expect(hbl.Items[0].Annotations).To(Equal(ans))
				Expect(hbl.Items[0].Labels).To(Equal(labels))

				deleteResource(lookupKey(chb), chb)
				Expect(k8sClient.DeleteAllOf(context.Background(), &hazelcastv1alpha1.HotBackup{}, client.InNamespace(namespace))).Should(Succeed())
			})
		})
	})
})
