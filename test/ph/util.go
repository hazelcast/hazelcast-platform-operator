package ph

import (
	"cloud.google.com/go/bigquery"
	"context"
	. "github.com/onsi/gomega"
	"google.golang.org/api/iterator"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

const (
	hzName = "hazelcast"
	mcName = "managementcenter"
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
	AverageClusterCreationLatency bigquery.NullInt64 `bigquery:"averageClusterCreationLatency"`
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
		`SELECT * FROM ` + bigQueryTable() + `
                WHERE pingTime = (
                SELECT max(pingTime) from ` + bigQueryTable() + `
                WHERE operatorID =  "` + getOperatorId() + `");`)
	return query.Read(ctx)
}

func getBigQueryTable() OperatorPhoneHome {

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

func emptyManagementCenter() *hazelcastcomv1alpha1.ManagementCenter {
	return &hazelcastcomv1alpha1.ManagementCenter{
		ObjectMeta: v1.ObjectMeta{
			Name:      mcName,
			Namespace: hzNamespace,
		},
	}
}

func useExistingCluster() bool {
	return strings.ToLower(os.Getenv("USE_EXISTING_CLUSTER")) == "true"
}

func runningLocally() bool {
	return strings.ToLower(os.Getenv("RUN_MANAGER_LOCALLY")) == "true"
}
func assertDoesNotExist(name types.NamespacedName, obj client.Object) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), name, obj)
		if err == nil {
			return false
		}
		return errors.IsNotFound(err)
	}, deleteTimeout, interval).Should(BeTrue())
}
func controllerManagerName() string {
	np := os.Getenv("NAME_PREFIX")
	if np == "" {
		return "hazelcast-platform-controller-manager"
	}
	return np + "controller-manager"
}

func bigQueryTable() string {
	bigQueryTableName := os.Getenv("BIG_QUERY_TABLE")
	if bigQueryTableName == "" {
		return "hazelcast-33.callHome.operator_info"
	}
	return bigQueryTableName
}

func googleCloudProjectName() string {
	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		return "hazelcast-33"
	}
	return projectID
}

func getKubectlVersion() string {
	kubectlVersion := os.Getenv("KUBECTL_VERSION")
	if kubectlVersion == "" {
		return "1.21"
	}
	return kubectlVersion
}

func getDeploymentReadyReplicas(ctx context.Context, name types.NamespacedName, deploy *appsv1.Deployment) (int32, error) {
	err := k8sClient.Get(ctx, name, deploy)
	if err != nil {
		if errors.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}

	return deploy.Status.ReadyReplicas, nil
}

func isManagementCenterRunning(mc *hazelcastcomv1alpha1.ManagementCenter) bool {
	return mc.Status.Phase == "Running"
}

func deleteIfExists(name types.NamespacedName, obj client.Object) {
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), name, obj)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}

		return k8sClient.Delete(context.Background(), obj)
	}, timeout, interval).Should(Succeed())
}
