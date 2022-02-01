package ph

import (
	"cloud.google.com/go/bigquery"
	"context"
	"google.golang.org/api/iterator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

const (
	hzName        = "hazelcast"
	bigQueryTable = "hazelcast-33.callHome.operator_info"
)

type OperatorPhoneHome struct {
	IP                            string             `bigquery:"ip"`
	PingTime                      time.Time          `bigquery:"pingTime"`
	OperatorID                    string             `bigquery:"operatorID"`
	PardotID                      string             `bigquery:"pardotID"`
	Version                       string             `bigquery:"version"`
	Uptime                        int                `bigquery:"uptime"`
	K8sDistribution               string             `bigquery:"k8sDistribution"`
	K8sVersion                    string             `bigquery:"k8sVersion"`
	CreatedClusterCount           int                `bigquery:"createdClusterCount"`
	CreatedEnterpriseClusterCount int                `bigquery:"createdEnterpriseClusterCount"`
	AverageClusterCreationLatency int                `bigquery:"averageClusterCreationLatency"`
	AverageMCCreationLatency      bigquery.NullInt64 `bigquery:"averageMCCreationLatency"`
	CreatedMemberCount            int                `bigquery:"createdMemberCount"`
	CreatedMCCount                int                `bigquery:"createdMCCount"`
	ExposeExternally              ExposeExternally   `bigquery:"exposeExternally"`
}

type ExposeExternally struct {
	Unisocket                int `bigquery:"unisocket"`
	Smart                    int `bigquery:"smart"`
	DiscoveryLoadBalancer    int `bigquery:"discoveryLoadBalancer"`
	DiscoveryNodePort        int `bigquery:"discoveryNodePort"`
	MemberNodePortExternalIP int `bigquery:"memberNodePortExternalIP"`
	MemberNodePortNodeName   int `bigquery:"memberNodePortNodeName"`
	MemberLoadBalancer       int `bigquery:"memberLoadBalancer"`
}

var _ = Describe("Hazelcast", func() {

	var lookupKey = types.NamespacedName{
		Name:      hzName,
		Namespace: hzNamespace,
	}

	var controllerManagerName = types.NamespacedName{
		Name:      controllerManagerName(),
		Namespace: hzNamespace,
	}

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
			}, timeout, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), emptyHazelcast(), client.PropagationPolicy(v1.DeletePropagationForeground))).Should(Succeed())
		assertDoesNotExist(lookupKey, &hazelcastcomv1alpha1.Hazelcast{})
	})

	create := func(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
		By("Creating Hazelcast", func() {
			Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		})

		By("Checking Hazelcast running", func() {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return isHazelcastRunning(hz)
			}, timeout, interval).Should(BeTrue())
		})
	}

	evaluateReadyMembers := func(h *hazelcastcomv1alpha1.Hazelcast) {
		hz := &hazelcastcomv1alpha1.Hazelcast{}
		Eventually(func() string {
			err := k8sClient.Get(context.Background(), lookupKey, hz)
			Expect(err).ToNot(HaveOccurred())
			return hz.Status.Cluster.ReadyMembers
		}, timeout, interval).Should(Equal("3/3"))
	}

	Describe("Phone Home table", func() {

		It("with ExposeExternallySmartLoadBalancer configuration for Hazelcast should have metrics", func() {
			h := hazelcastconfig.ExposeExternallySmartLoadBalancer(hzNamespace, ee)
			create(h)
			hzCreationTime := time.Now().UTC().Truncate(time.Hour)
			evaluateReadyMembers(h)

			bigQueryTable := getBigQUeryTable()
			Expect(bigQueryTable.IP).Should(MatchRegexp("^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"), "IP address should be present and match regexp")
			Expect(bigQueryTable.PingTime.Truncate(time.Hour)).Should(BeTemporally("~", hzCreationTime), "Ping time should be near to current date")
			Expect(bigQueryTable.OperatorID).Should(Equal(getOperatorId()), "Operator UID should be equal to Hazelcast Operator UID")
			Expect(bigQueryTable.PardotID).Should(Equal("dockerhub"), "Pardot ID should be equal to dockerhub")
			Expect(bigQueryTable.Version).Should(Equal("latest-snapshot"))
			Expect(bigQueryTable.Uptime).ShouldNot(BeZero())
			Expect(bigQueryTable.K8sDistribution).Should(Equal("GKE"))
			Expect(bigQueryTable.K8sVersion).Should(Equal("1.21"))
			Expect(bigQueryTable.CreatedClusterCount).Should(Equal(0))
			Expect(bigQueryTable.CreatedEnterpriseClusterCount).Should(Equal(1))
			Expect(bigQueryTable.AverageClusterCreationLatency).ShouldNot(BeZero())
			Expect(bigQueryTable.AverageMCCreationLatency).Should(Equal(bigquery.NullInt64{}))
			Expect(bigQueryTable.CreatedMemberCount).Should(Equal(3))
			Expect(bigQueryTable.CreatedMCCount).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.Unisocket).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.Smart).Should(Equal(1))
			Expect(bigQueryTable.ExposeExternally.DiscoveryLoadBalancer).Should(Equal(1))
			Expect(bigQueryTable.ExposeExternally.DiscoveryNodePort).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.MemberNodePortExternalIP).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.MemberNodePortNodeName).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.MemberLoadBalancer).Should(Equal(1))
		})
		It("with ExposeExternallyUnisocket configuration for Hazelcast should have metrics", func() {
			h := hazelcastconfig.ExposeExternallyUnisocket(hzNamespace, ee)
			create(h)
			hzCreationTime := time.Now().UTC().Truncate(time.Hour)
			evaluateReadyMembers(h)
			bigQueryTable := getBigQUeryTable()

			Expect(bigQueryTable.IP).Should(MatchRegexp("^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"))
			Expect(bigQueryTable.PingTime.Truncate(time.Hour)).Should(BeTemporally("~", hzCreationTime))
			Expect(bigQueryTable.OperatorID).Should(Equal(getOperatorId()))
			Expect(bigQueryTable.PardotID).Should(Equal("dockerhub"))
			Expect(bigQueryTable.Version).Should(Equal("latest-snapshot"))
			Expect(bigQueryTable.Uptime).ShouldNot(BeZero())
			Expect(bigQueryTable.K8sDistribution).Should(Equal("GKE"))
			Expect(bigQueryTable.K8sVersion).Should(Equal("1.21"))
			Expect(bigQueryTable.CreatedClusterCount).Should(Equal(0))
			Expect(bigQueryTable.CreatedEnterpriseClusterCount).Should(Equal(1))
			Expect(bigQueryTable.AverageClusterCreationLatency).ShouldNot(BeZero())
			Expect(bigQueryTable.AverageMCCreationLatency).Should(Equal(bigquery.NullInt64{}))
			Expect(bigQueryTable.CreatedMemberCount).Should(Equal(3))
			Expect(bigQueryTable.CreatedMCCount).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.Unisocket).Should(Equal(1))
			Expect(bigQueryTable.ExposeExternally.Smart).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.DiscoveryLoadBalancer).Should(Equal(1))
			Expect(bigQueryTable.ExposeExternally.DiscoveryNodePort).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.MemberNodePortExternalIP).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.MemberNodePortNodeName).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.MemberLoadBalancer).Should(Equal(0))
		})

		FIt("with ExposeExternallySmartNodePort configuration should have metrics", func() {
			h := hazelcastconfig.ExposeExternallySmartNodePort(hzNamespace, ee)
			create(h)
			hzCreationTime := time.Now().UTC().Truncate(time.Hour)
			evaluateReadyMembers(h)
			bigQueryTable := getBigQUeryTable()
			Expect(bigQueryTable.IP).Should(MatchRegexp("^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"))
			Expect(bigQueryTable.PingTime.Truncate(time.Hour)).Should(BeTemporally("~", hzCreationTime))
			Expect(bigQueryTable.OperatorID).Should(Equal(getOperatorId()))
			Expect(bigQueryTable.PardotID).Should(Equal("dockerhub"))
			Expect(bigQueryTable.Version).Should(Equal("latest-snapshot"))
			Expect(bigQueryTable.Uptime).ShouldNot(BeZero())
			Expect(bigQueryTable.K8sDistribution).Should(Equal("GKE"))
			Expect(bigQueryTable.K8sVersion).Should(Equal("1.21"))
			Expect(bigQueryTable.CreatedClusterCount).Should(Equal(0))
			Expect(bigQueryTable.CreatedEnterpriseClusterCount).Should(Equal(1))
			Expect(bigQueryTable.AverageClusterCreationLatency).ShouldNot(BeZero())
			Expect(bigQueryTable.AverageMCCreationLatency).Should(Equal(bigquery.NullInt64{}))
			Expect(bigQueryTable.CreatedMemberCount).Should(Equal(3))
			Expect(bigQueryTable.CreatedMCCount).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.Unisocket).Should(Equal(0), "Unisocket column should be equal to 1")
			Expect(bigQueryTable.ExposeExternally.Smart).Should(Equal(1), "Smart column should be equal to 1")
			Expect(bigQueryTable.ExposeExternally.DiscoveryLoadBalancer).Should(Equal(1), "DiscoveryLoadBalancer column should be equal to 1")
			Expect(bigQueryTable.ExposeExternally.DiscoveryNodePort).Should(Equal(0), "DiscoveryNodePort column should be equal to 0")
			Expect(bigQueryTable.ExposeExternally.MemberNodePortExternalIP).Should(Equal(1), "MemberNodePortExternalIP column should be equal to 1") //bug here
			Expect(bigQueryTable.ExposeExternally.MemberNodePortNodeName).Should(Equal(0), "MemberNodePortNodeName column should be equal to 0")
			Expect(bigQueryTable.ExposeExternally.MemberLoadBalancer).Should(Equal(0), "MemberLoadBalancer column should be equal to 0")

		})

		It("with ExposeExternallySmartLoadBalancer configuration", func() {
			h := hazelcastconfig.ExposeExternallySmartLoadBalancer(hzNamespace, ee)
			create(h)
			hzCreationTime := time.Now().UTC().Truncate(time.Hour)
			evaluateReadyMembers(h)
			bigQueryTable := getBigQUeryTable()
			Expect(bigQueryTable.IP).Should(MatchRegexp("^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"))
			Expect(bigQueryTable.PingTime.Truncate(time.Hour)).Should(BeTemporally("~", hzCreationTime))
			Expect(bigQueryTable.OperatorID).Should(Equal(getOperatorId()))
			Expect(bigQueryTable.PardotID).Should(Equal("dockerhub"))
			Expect(bigQueryTable.Version).Should(Equal("latest-snapshot"))
			Expect(bigQueryTable.Uptime).ShouldNot(BeZero())
			Expect(bigQueryTable.K8sDistribution).Should(Equal("GKE"))
			Expect(bigQueryTable.K8sVersion).Should(Equal("1.21"))
			Expect(bigQueryTable.CreatedClusterCount).Should(Equal(0))
			Expect(bigQueryTable.CreatedEnterpriseClusterCount).Should(Equal(1))
			Expect(bigQueryTable.AverageClusterCreationLatency).ShouldNot(BeZero())
			Expect(bigQueryTable.AverageMCCreationLatency).Should(Equal(bigquery.NullInt64{}))
			Expect(bigQueryTable.CreatedMemberCount).Should(Equal(3))
			Expect(bigQueryTable.CreatedMCCount).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.Unisocket).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.Smart).Should(Equal(1))
			Expect(bigQueryTable.ExposeExternally.DiscoveryLoadBalancer).Should(Equal(1))
			Expect(bigQueryTable.ExposeExternally.DiscoveryNodePort).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.MemberNodePortExternalIP).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.MemberNodePortNodeName).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.MemberLoadBalancer).Should(Equal(1))
		})

		It("with ExposeExternallySmartNodePortNodeName configuration", func() {
			h := hazelcastconfig.ExposeExternallySmartNodePortNodeName(hzNamespace, ee)
			create(h)
			hzCreationTime := time.Now().UTC().Truncate(time.Hour)
			evaluateReadyMembers(h)
			bigQueryTable := getBigQUeryTable()
			Expect(bigQueryTable.IP).Should(MatchRegexp("^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"))
			Expect(bigQueryTable.PingTime.Truncate(time.Hour)).Should(BeTemporally("~", hzCreationTime))
			Expect(bigQueryTable.OperatorID).Should(Equal(getOperatorId()))
			Expect(bigQueryTable.PardotID).Should(Equal("dockerhub"))
			Expect(bigQueryTable.Version).Should(Equal("latest-snapshot"))
			Expect(bigQueryTable.Uptime).ShouldNot(BeZero())
			Expect(bigQueryTable.K8sDistribution).Should(Equal("GKE"))
			Expect(bigQueryTable.K8sVersion).Should(Equal("1.21"))
			Expect(bigQueryTable.CreatedClusterCount).Should(Equal(0))
			Expect(bigQueryTable.CreatedEnterpriseClusterCount).Should(Equal(1))
			Expect(bigQueryTable.AverageClusterCreationLatency).ShouldNot(BeZero())
			Expect(bigQueryTable.AverageMCCreationLatency).Should(Equal(bigquery.NullInt64{}))
			Expect(bigQueryTable.CreatedMemberCount).Should(Equal(3))
			Expect(bigQueryTable.CreatedMCCount).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.Unisocket).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.Smart).Should(Equal(1))
			Expect(bigQueryTable.ExposeExternally.DiscoveryLoadBalancer).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.DiscoveryNodePort).Should(Equal(1))
			Expect(bigQueryTable.ExposeExternally.MemberNodePortExternalIP).Should(Equal(0))
			Expect(bigQueryTable.ExposeExternally.MemberNodePortNodeName).Should(Equal(1))
			Expect(bigQueryTable.ExposeExternally.MemberLoadBalancer).Should(Equal(0))
		})
	})

})

func emptyHazelcast() *hazelcastcomv1alpha1.Hazelcast {
	return &hazelcastcomv1alpha1.Hazelcast{
		ObjectMeta: v1.ObjectMeta{
			Name:      hzName,
			Namespace: hzNamespace,
		},
	}
}
func isHazelcastRunning(hz *hazelcastcomv1alpha1.Hazelcast) bool {
	return hz.Status.Phase == "Running"
}

func GetClientSet() *kubernetes.Clientset {
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	restConfig, err := kubeConfig.ClientConfig()
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatal(err)
	}
	return clientSet
}
func getOperatorId() string {
	var uid string
	operatorUid, _ := GetClientSet().AppsV1().Deployments(hzNamespace).List(context.Background(), metav1.ListOptions{})
	for _, item := range operatorUid.Items {
		if item.Name == controllerManagerName() {
			uid = string(item.UID)
		}
	}
	return uid
}

func query(ctx context.Context, client *bigquery.Client) (*bigquery.RowIterator, error) {
	query := client.Query(
		`SELECT * FROM ` + bigQueryTable + `
                WHERE pingTime = (
                SELECT max(pingTime) from ` + bigQueryTable + `
                WHERE operatorID =  "` + getOperatorId() + `");`)
	return query.Read(ctx)
}

func getBigQUeryTable() OperatorPhoneHome {

	ctx := context.Background()
	bigQueryclient, err := bigquery.NewClient(ctx, googleCloudProjectName())
	if err != nil {
		log.Fatalf("bigquery.NewClient: %v", err)
	}
	defer bigQueryclient.Close()

	rows, err := query(ctx, bigQueryclient)
	var row OperatorPhoneHome
	rows.Next(&row)
	if err == iterator.Done {
	}
	return row

}
