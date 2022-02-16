package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	turbineconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/turbine"
)

var _ = Describe("Turbine", func() {

	var hzLookupKey = types.NamespacedName{
		Name:      hzName,
		Namespace: hzNamespace,
	}

	var controllerManagerName = types.NamespacedName{
		Name:      controllerManagerName(),
		Namespace: hzNamespace,
	}

	var testLabels = map[string]string{
		"test": "true",
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
		ctx := context.Background()
		Expect(k8sClient.Delete(ctx, emptyHazelcast(), client.PropagationPolicy(metav1.DeletePropagationForeground))).Should(Succeed())
		Expect(k8sClient.Delete(ctx, turbineconfig.PingPong(hzNamespace))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(
			ctx,
			&v1.Service{},
			client.MatchingLabels(testLabels),
			client.PropagationPolicy(metav1.DeletePropagationForeground)),
		).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(
			ctx,
			&appsv1.Deployment{},
			client.MatchingLabels(testLabels),
			client.PropagationPolicy(metav1.DeletePropagationForeground)),
		).Should(Succeed())
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
	})

	createHz := func(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
		By("Creating Hazelcast CR", func() {
			Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		})

		By("Checking Hazelcast CR running", func() {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), hzLookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return isHazelcastRunning(hz)
			}, timeout, interval).Should(BeTrue())
		})
	}

	getSidecar := func(containers []v1.Container, t *hazelcastcomv1alpha1.Turbine) (*v1.Container, error) {
		for i, c := range containers {
			if t.Spec.Sidecar.Name == c.Name && t.Spec.Sidecar.Image == c.Image {
				return &containers[i], nil
			}
		}
		return nil, errors.New("could not find the sidecar")
	}

	checkSidecar := func(d *appsv1.Deployment, t *hazelcastcomv1alpha1.Turbine) {
		labels := d.Spec.Selector.MatchLabels
		list := v1.PodList{}
		Expect(k8sClient.List(context.Background(), &list, client.MatchingLabels(labels), client.InNamespace(d.Namespace))).Should(Succeed())
		for _, p := range list.Items {
			Expect(p.Spec.Containers).Should(HaveLen(2))
			s, err := getSidecar(p.Spec.Containers, t)
			Expect(err).Should(Succeed())

			Expect(s.Env).Should(Not(ContainElement(HaveField("Value", ""))))
			Expect(s.Env).Should(ContainElement(HaveField("Name", "CLUSTER_ADDRESS")))
			Expect(s.Env).Should(ContainElement(HaveField("Name", "APP_HTTP_PORT")))
			Expect(s.Env).Should(ContainElement(HaveField("Name", "TURBINE_POD_IP")))
		}

		Eventually(func() bool {
			depl := appsv1.Deployment{}
			key := types.NamespacedName{Name: depl.Name, Namespace: depl.Namespace}
			Expect(k8sClient.Get(context.Background(), key, &depl)).Should(Succeed())
			return depl.Status.ReadyReplicas == depl.Status.Replicas
		}).Should(BeTrue())
	}

	waitForExternalAddress := func(svc *v1.Service) string {
		lb := v1.Service{}
		key := types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}
		addr := ""
		Eventually(func() string {
			Expect(k8sClient.Get(context.Background(), key, &lb)).To(Succeed())
			if len(lb.Status.LoadBalancer.Ingress) == 0 {
				return ""
			}
			if lb.Status.LoadBalancer.Ingress[0].IP != "" {
				addr = lb.Status.LoadBalancer.Ingress[0].IP
			} else if lb.Status.LoadBalancer.Ingress[0].Hostname != "" {
				addr = lb.Status.LoadBalancer.Ingress[0].Hostname
			}
			if addr == "" {
				return ""
			}
			addr = fmt.Sprintf("%s:%d", addr, lb.Spec.Ports[0].Port)
			return addr
		}, timeout, interval).Should(Not(BeEmpty()))
		return addr
	}

	Describe("Run Ping Pong", func() {
		hazelcast := hazelcastconfig.ExposeExternallyUnisocket(hzNamespace, ee)
		createHz(hazelcast)

		turbine := turbineconfig.PingPong(hzNamespace)
		ping := turbineconfig.PingDeployment(hzNamespace)
		pong := turbineconfig.PongDeployment(hzNamespace)
		pingServ := turbineconfig.PingService(hzNamespace)
		pongServ := turbineconfig.PongService(hzNamespace)
		Expect(k8sClient.Create(context.Background(), turbine)).Should(Succeed())
		Expect(k8sClient.Create(context.Background(), ping)).Should(Succeed())
		Expect(k8sClient.Create(context.Background(), pingServ)).Should(Succeed())
		Expect(k8sClient.Create(context.Background(), pong)).Should(Succeed())
		Expect(k8sClient.Create(context.Background(), pongServ)).Should(Succeed())

		checkSidecar(ping, turbine)
		checkSidecar(pong, turbine)

		externalAddr := waitForExternalAddress(pingServ)
		resp, err := http.Get(externalAddr + "/do-ping/pong")
		Expect(err).Should(Succeed())

		Expect(io.ReadAll(resp.Body)).Should(ContainSubstring("I sent ping and got back PONG!"))
		Expect(resp.Body.Close()).Should(Succeed())
	})
})
