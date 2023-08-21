package ph

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	. "time"

	"cloud.google.com/go/bigquery"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/api/iterator"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type OperatorPhoneHome struct {
	IP                            string             `bigquery:"ip"`
	PingTime                      Time               `bigquery:"pingTime"`
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

func isHazelcastRunning(hz *hazelcastcomv1alpha1.Hazelcast) bool {
	return hz.Status.Phase == "Running"
}

func GetClientSet() *kubernetes.Clientset {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	restConfig, _ := kubeConfig.ClientConfig()
	clientSet := kubernetes.NewForConfigOrDie(restConfig)
	return clientSet
}

func getOperatorId() string {
	clientSet := GetClientSet()
	deploymentsClient := clientSet.AppsV1().Deployments(hzNamespace)
	deployment, err := deploymentsClient.Get(context.Background(), GetControllerManagerName(), metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}
	GinkgoWriter.Printf("Operator ID is: %s\n", deployment.UID)
	return string(deployment.UID)
}

func GetSuiteName() string {
	if !ee {
		return "Operator Suite OS"
	}
	return "Operator Suite EE"
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
	var row OperatorPhoneHome
	By("getting BigQuery table", func() {
		ctx := context.Background()
		bigQueryclient, err := bigquery.NewClient(ctx, googleCloudProjectName())
		if err != nil {
			log.Fatalf("bigquery.NewClient: %v", err)
		}
		defer bigQueryclient.Close()

		rows, err := query(ctx, bigQueryclient)
		if err != nil {
			log.Fatal(err)
		}
		for {
			err := rows.Next(&row)
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Fatalf("Error iterating over rows: %v", err)
			}
		}
	})
	return row
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

func GetControllerManagerName() string {
	return os.Getenv("DEPLOYMENT_NAME")
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

func isManagementCenterRunning(mc *hazelcastcomv1alpha1.ManagementCenter) bool {
	return mc.Status.Phase == "Running"
}

func DeleteAllOf(obj client.Object, objList client.ObjectList, ns string, labels map[string]string) {
	Expect(k8sClient.DeleteAllOf(
		context.Background(),
		obj,
		client.InNamespace(ns),
		client.MatchingLabels(labels),
		client.PropagationPolicy(metav1.DeletePropagationForeground),
	)).Should(Succeed())

	// do not wait if objList is nil
	objListVal := reflect.ValueOf(objList)
	if !objListVal.IsValid() {
		return
	}

	Eventually(func() int {
		err := k8sClient.List(context.Background(), objList,
			client.InNamespace(ns),
			client.MatchingLabels(labels))
		if err != nil {
			return -1
		}
		if objListVal.Kind() == reflect.Ptr || objListVal.Kind() == reflect.Interface {
			objListVal = objListVal.Elem()
		}
		items := objListVal.FieldByName("Items")
		len := items.Len()
		return len

	}, 2*Minute, interval).Should(Equal(int(0)))
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

func evaluateReadyMembers(lookupKey types.NamespacedName) {
	By(fmt.Sprintf("evaluate number of ready members for lookup name '%s' and '%s' namespace", lookupKey.Name, lookupKey.Namespace), func() {
		hz := &hazelcastcomv1alpha1.Hazelcast{}
		err := k8sClient.Get(context.Background(), lookupKey, hz)
		Expect(err).ToNot(HaveOccurred())
		membersCount := int(*hz.Spec.ClusterSize)
		Eventually(func() string {
			err := k8sClient.Get(context.Background(), lookupKey, hz)
			Expect(err).ToNot(HaveOccurred())
			return hz.Status.Cluster.ReadyMembers
		}, 6*Minute, interval).Should(Equal(fmt.Sprintf("%d/%d", membersCount, membersCount)))
	})
}

func CreateHazelcastCR(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	By("creating Hazelcast CR", func() {
		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
	})
	lk := types.NamespacedName{Name: hazelcast.Name, Namespace: hazelcast.Namespace}
	message := ""
	By("checking Hazelcast CR running", func() {
		hz := &hazelcastcomv1alpha1.Hazelcast{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), lk, hz)
			if err != nil {
				return false
			}
			message = hz.Status.Message
			return isHazelcastRunning(hz)
		}, 10*Minute, interval).Should(BeTrue(), "Message: %v", message)
	})
}

func CreateMC(mancenter *hazelcastcomv1alpha1.ManagementCenter) {
	By("creating ManagementCenter CR", func() {
		Expect(k8sClient.Create(context.Background(), mancenter)).Should(Succeed())
	})

	By("checking ManagementCenter CR running", func() {
		mc := &hazelcastcomv1alpha1.ManagementCenter{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), mcLookupKey, mc)
			Expect(err).ToNot(HaveOccurred())
			return isManagementCenterRunning(mc)
		}, 5*Minute, interval).Should(BeTrue())
	})
}
