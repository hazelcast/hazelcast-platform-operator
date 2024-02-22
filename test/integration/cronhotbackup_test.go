package integration

import (
	"context"
	"fmt"
	"strings"
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

	filterHotBackupsByName := func(chb hazelcastv1alpha1.CronHotBackup, hbs []hazelcastv1alpha1.HotBackup) []hazelcastv1alpha1.HotBackup {
		fhbs := make([]hazelcastv1alpha1.HotBackup, 0)
		for _, hb := range hbs {
			if strings.HasPrefix(hb.Name, chb.GetName()) {
				fhbs = append(fhbs, hb)
			}
		}
		return fhbs
	}

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.CronHotBackup{}, nil, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.HotBackup{}, nil, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("with default configuration", func() {
		It("should create successfully", func() {
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
		})

		When("applying empty spec", func() {
			It("should fail to create", func() {
				chb := &hazelcastv1alpha1.CronHotBackup{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create CronHotBackup CR")
				Expect(k8sClient.Create(context.Background(), chb)).ShouldNot(Succeed())
				assertDoesNotExist(lookupKey(chb), chb)
			})
		})

		It("should handle HotBackup resource correctly", func() {
			chb := &hazelcastv1alpha1.CronHotBackup{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.CronHotBackupSpec{
					Schedule: "*/1 * * * * *",
					HotBackupTemplate: hazelcastv1alpha1.HotBackupTemplateSpec{
						Spec: hazelcastv1alpha1.HotBackupSpec{
							HazelcastResourceName: "hazelcast",
							BucketURI:             "s3://bucket-name/path/to/folder",
							SecretName:            "bucket-secret",
						},
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
				err := k8sClient.List(context.Background(), hbl, client.InNamespace(namespace))
				if err != nil {
					return ""
				}
				if len(hbl.Items) < 1 {
					return ""
				}
				filteredHbs := filterHotBackupsByName(*chb, hbl.Items)
				if len(filteredHbs) == 0 {
					return ""
				}
				return filteredHbs[0].Name
			}, timeout, interval).Should(ContainSubstring(chb.Name))

			for _, hb := range hbl.Items {
				Expect(hb.Spec.HazelcastResourceName).To(Equal("hazelcast"))
				Expect(hb.Spec.BucketURI).To(Equal("s3://bucket-name/path/to/folder"))
				Expect(hb.Spec.SecretName).To(Equal("bucket-secret"))
			}
		})

		When("giving labels and annotations to HotBackup Template", func() {
			It("should create successfully with those labels", func() {
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
				var hotBackupItem *hazelcastv1alpha1.HotBackup
				Eventually(func() string {
					hbl := &hazelcastv1alpha1.HotBackupList{}
					err := k8sClient.List(context.Background(), hbl, client.InNamespace(namespace), client.MatchingLabels(labels))
					if err != nil {
						return ""
					}
					if len(hbl.Items) < 1 {
						return ""
					}
					filteredHbs := filterHotBackupsByName(*chb, hbl.Items)
					if len(filteredHbs) == 0 {
						return ""
					}
					hotBackupItem = &filteredHbs[0]
					return filteredHbs[0].Name
				}, timeout, interval).Should(ContainSubstring(chb.Name))

				Expect(hotBackupItem.Annotations).To(Equal(ans))
				Expect(hotBackupItem.Labels).To(Equal(labels))
			})
		})
	})
})
