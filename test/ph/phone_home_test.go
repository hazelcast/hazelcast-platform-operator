package ph

import (
	"context"
	"fmt"
	"time"
	. "time"

	"cloud.google.com/go/bigquery"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	mcconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/managementcenter"
)

var _ = Describe("Hazelcast", func() {

	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
		By("Checking hazelcast-platform-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
	})

	assertAnnotationExists := func(obj client.Object) {
		cpy, ok := obj.DeepCopyObject().(client.Object)
		if !ok {
			Fail("Failed copying client.Object.")
		}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: cpy.GetName(), Namespace: cpy.GetNamespace()}, cpy)
			if err != nil {
				return false
			}
			_, ok := cpy.GetAnnotations()[n.LastSuccessfulSpecAnnotation]
			return ok
		}, timeout, interval).Should(BeTrue())
	}

	Describe("Phone Home Table with installed Hazelcast", func() {
		AfterEach(func() {
			DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, &hazelcastcomv1alpha1.HazelcastList{}, hzNamespace, nil)
			assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		})

		DescribeTable("should have correct metrics",
			func(config string, createdEnterpriseClusterCount int, unisocket int, smart int, discoveryLoadBalancer int, discoveryNodePort int, memberNodePortExternalIP int, memberNodePortNodeName int, memberLoadBalancer int) {
				setLabelAndCRName("phhz")
				var cfg *hazelcastcomv1alpha1.Hazelcast
				switch config {
				case "unisocket":
					cfg = hazelcastconfig.ExposeExternallyUnisocket(hzLookupKey, ee, nil)
				case "smartNodePort":
					cfg = hazelcastconfig.ExposeExternallySmartNodePort(hzLookupKey, ee, nil)
				case "smartLoadBalancer":
					cfg = hazelcastconfig.ExposeExternallySmartLoadBalancer(hzLookupKey, ee, nil)
				case "smartNodePortNodeName":
					cfg = hazelcastconfig.ExposeExternallySmartNodePortNodeName(hzLookupKey, ee, nil)
				default:
					Fail("Incorrect input configuration")
				}
				CreateHazelcastCR(cfg)
				hzCreationTime := time.Now().UTC().Truncate(time.Hour)
				evaluateReadyMembers(hzLookupKey, 3)
				assertAnnotationExists(cfg)
				bigQueryTable := getBigQueryTable()
				Expect(bigQueryTable.IP).Should(MatchRegexp("^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"), "IP address should be present and match regexp")
				Expect(bigQueryTable.PingTime.Truncate(time.Hour)).Should(BeTemporally("~", hzCreationTime), "Ping time should be near to current date")
				Expect(bigQueryTable.OperatorID).Should(Equal(getOperatorId()), "Operator UID metric")
				Expect(bigQueryTable.PardotID).Should(Equal("dockerhub"), "Pardot ID metric")
				Expect(bigQueryTable.Version).Should(Equal(version), "Version metric")
				Expect(bigQueryTable.Uptime).ShouldNot(BeZero(), "Version metric")
				Expect(bigQueryTable.K8sDistribution).Should(Equal("GKE"), "K8sDistribution metric")
				Expect(bigQueryTable.K8sVersion).ShouldNot(BeEmpty(), "K8sVersion metric")
				Expect(bigQueryTable.CreatedClusterCount).Should(Equal(0), "CreatedClusterCount metric")
				Expect(bigQueryTable.CreatedEnterpriseClusterCount).Should(Equal(createdEnterpriseClusterCount), "CreatedEnterpriseClusterCount metric")
				Expect(bigQueryTable.AverageClusterCreationLatency).Should(Equal(bigquery.NullInt64{}), "AverageClusterCreationLatency metric")
				Expect(bigQueryTable.AverageMCCreationLatency).Should(Equal(bigquery.NullInt64{}), "AverageMCCreationLatency metric")
				Expect(bigQueryTable.CreatedMemberCount).Should(Equal(3), "CreatedMemberCount metric")
				Expect(bigQueryTable.CreatedMCCount).Should(Equal(0), "CreatedMCCount metric")
				Expect(bigQueryTable.ExposeExternally.Unisocket).Should(Equal(unisocket), "Unisocket metric")
				Expect(bigQueryTable.ExposeExternally.Smart).Should(Equal(smart), "Smart metric")
				Expect(bigQueryTable.ExposeExternally.DiscoveryLoadBalancer).Should(Equal(discoveryLoadBalancer), "DiscoveryLoadBalancer metric")
				Expect(bigQueryTable.ExposeExternally.DiscoveryNodePort).Should(Equal(discoveryNodePort), "DiscoveryNodePort metric")
				Expect(bigQueryTable.ExposeExternally.MemberNodePortExternalIP).Should(Equal(memberNodePortExternalIP), "MemberNodePortExternalIP metric")
				Expect(bigQueryTable.ExposeExternally.MemberNodePortNodeName).Should(Equal(memberNodePortNodeName), "MemberNodePortNodeName metric")
				Expect(bigQueryTable.ExposeExternally.MemberLoadBalancer).Should(Equal(memberLoadBalancer), "MemberLoadBalancer metric")
			},
			Entry("with ExposeExternallyUnisocket configuration", Label("slow"), "unisocket", 1, 1, 0, 1, 0, 0, 0, 0),
			Entry("with ExposeExternallySmartNodePort configuration", Label("slow"), "smartNodePort", 1, 0, 1, 1, 0, 1, 0, 0),
			Entry("with ExposeExternallySmartLoadBalancer configuration", Label("slow"), "smartLoadBalancer", 1, 0, 1, 1, 0, 0, 0, 1),
			Entry("with ExposeExternallySmartNodePortNodeName configuration", Label("fast"), "smartNodePortNodeName", 1, 0, 1, 0, 1, 0, 1, 0),
		)
	})
	Describe("Phone Home table with installed Management Center", func() {
		AfterEach(func() {
			DeleteAllOf(&hazelcastcomv1alpha1.ManagementCenter{}, &hazelcastcomv1alpha1.ManagementCenterList{}, hzNamespace, labels)
			assertDoesNotExist(mcLookupKey, &hazelcastcomv1alpha1.ManagementCenter{})
			pvcLookupKey := types.NamespacedName{
				Name:      fmt.Sprintf("mancenter-storage-%s-0", mcLookupKey.Name),
				Namespace: mcLookupKey.Namespace,
			}
			deleteIfExists(pvcLookupKey, &corev1.PersistentVolumeClaim{})
		})
		It("should have correct metrics", Label("fast"), func() {
			setLabelAndCRName("phmc")
			mc := mcconfig.Default(mcLookupKey, ee, labels)
			CreateMC(mc)
			mcCreationTime := time.Now().Truncate(time.Hour)
			assertAnnotationExists(mc)
			bigQueryTable := getBigQueryTable()
			Expect(bigQueryTable.IP).Should(MatchRegexp("^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"), "IP address should be present and match regexp")
			Expect(bigQueryTable.PingTime.Truncate(time.Hour)).Should(BeTemporally("~", mcCreationTime), "Ping time should be near to current date")
			Expect(bigQueryTable.OperatorID).Should(Equal(getOperatorId()), "Operator UID should be equal to Hazelcast Operator UID")
			Expect(bigQueryTable.PardotID).Should(Equal("dockerhub"), "Pardot ID metric")
			Expect(bigQueryTable.Version).Should(Equal(version), "Version metric")
			Expect(bigQueryTable.Uptime).ShouldNot(BeZero(), "Uptime metric")
			Expect(bigQueryTable.K8sDistribution).Should(Equal("GKE"), "K8sDistribution metric")
			Expect(bigQueryTable.K8sVersion).ShouldNot(BeEmpty(), "K8sVersion metric")
			Expect(bigQueryTable.CreatedClusterCount).Should(Equal(0), "CreatedClusterCount metric")
			Expect(bigQueryTable.CreatedEnterpriseClusterCount).Should(Equal(0), "CreatedEnterpriseClusterCount metric")
			Expect(bigQueryTable.AverageClusterCreationLatency).Should(Equal(bigquery.NullInt64{}), "AverageClusterCreationLatency metric")
			Expect(bigQueryTable.AverageMCCreationLatency).Should(Equal(bigquery.NullInt64{}), "AverageMCCreationLatency metric")
			Expect(bigQueryTable.CreatedMemberCount).Should(Equal(0), "CreatedMemberCount metric")
			Expect(bigQueryTable.CreatedMCCount).Should(Equal(1), "CreatedMCCount metric")
			Expect(bigQueryTable.ExposeExternally.Unisocket).Should(Equal(0), "Unisocket metric")
			Expect(bigQueryTable.ExposeExternally.Smart).Should(Equal(0), "Smart metric")
			Expect(bigQueryTable.ExposeExternally.DiscoveryLoadBalancer).Should(Equal(0), "DiscoveryLoadBalancer metric")
			Expect(bigQueryTable.ExposeExternally.DiscoveryNodePort).Should(Equal(0), "DiscoveryNodePort metric")
			Expect(bigQueryTable.ExposeExternally.MemberNodePortExternalIP).Should(Equal(0), "MemberNodePortExternalIP metric")
			Expect(bigQueryTable.ExposeExternally.MemberNodePortNodeName).Should(Equal(0), "MemberNodePortNodeName metric")
			Expect(bigQueryTable.ExposeExternally.MemberLoadBalancer).Should(Equal(0), "MemberLoadBalancer metric")
		})
	})
})
