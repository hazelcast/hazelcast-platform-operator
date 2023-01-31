package e2e

import (
	"context"
	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	v1 "k8s.io/api/core/v1"
	"strings"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Hazelcast CR with Advanced Networking Feature", Label("hz_advanced_networking"), func() {
	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
		By("checking hazelcast-platform-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should create advanced network config", Label("slow"), func() {
		wanPort := 5710
		wanPortCount := 5
		interfaces := []string{"10.10.1.*"}

		setLabelAndCRName("hz-1")
		hz := hazelcastconfig.AdvancedNetwork(hzLookupKey, ee, labels,
			uint(wanPort), uint(wanPortCount), v1.ServiceTypeNodePort, interfaces)
		CreateHazelcastCR(hz)

		//TODO: check if hazelcast config created successfully

		// check if services created successfully
		serviceList := &v1.ServiceList{}
		err := k8sClient.List(context.Background(), serviceList, client.InNamespace(hz.Namespace))
		Expect(err).Should(BeNil())
		var wanRepSvcCount = 0
		for _, s := range serviceList.Items {
			if strings.Contains(s.Name, "-wan-rep-port-") {
				wanRepSvcCount++
			}
		}
		Expect(wanRepSvcCount).Should(Equal(wanPortCount))
	})
})