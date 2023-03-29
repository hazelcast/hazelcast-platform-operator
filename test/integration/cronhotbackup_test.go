package integration

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("CronHotBackup CR", func() {
	const namespace = "default"

	GetRandomObjectMeta := func() metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:      fmt.Sprintf("hazelcast-test-%s", uuid.NewUUID()),
			Namespace: namespace,
		}
	}

	Delete := func(obj client.Object) {
		By("expecting to delete CR successfully")
		deleteIfExists(lookupKey(obj), obj)
		By("expecting to CR delete finish")
		assertDoesNotExist(lookupKey(obj), obj)
	}

	DeleteWithoutWaiting := func(obj client.Object) {
		By("expecting to delete CR successfully")
		deleteIfExists(lookupKey(obj), obj)
	}

	Context("CronHotBackup CR configuration", func() {
		When("CronJob With Empty Spec is created", func() {
			It("Should fail to create", Label("fast"), func() {
				chb := &hazelcastv1alpha1.CronHotBackup{
					ObjectMeta: GetRandomObjectMeta(),
				}
				By("failing to create CronHotBackup CR")
				Expect(k8sClient.Create(context.Background(), chb)).ShouldNot(Succeed())
				Delete(chb)
			})
		})

		When("Using default configuration", func() {
			It("should create CronHotBackup with default values", Label("fast"), func() {
				chb := &hazelcastv1alpha1.CronHotBackup{
					ObjectMeta: GetRandomObjectMeta(),
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
				Delete(chb)
			})
		})

		When("Giving labels and annotations to HotBackup Template", func() {
			It("should HotBackup with those labels", Label("fast"), func() {
				ans := map[string]string{
					"annotation1": "val",
					"annotation2": "val2",
				}
				labels := map[string]string{
					"label1": "val",
					"label2": "val2",
				}
				chb := &hazelcastv1alpha1.CronHotBackup{
					ObjectMeta: GetRandomObjectMeta(),
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

				// Foreground deletion does not work in integration tests, should delete CronHotBackup without waiting
				DeleteWithoutWaiting(chb)
				Expect(k8sClient.DeleteAllOf(context.Background(), &hazelcastv1alpha1.HotBackup{}, client.InNamespace(namespace))).Should(Succeed())
			})
		})
	})
})
