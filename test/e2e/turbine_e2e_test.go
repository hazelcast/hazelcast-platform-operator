package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	turbineconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/turbine"
)

var _ = Describe("Turbine", func() {

	var hazelcast = turbineconfig.ExposeExternallyUnisocket(hzNamespace, ee)

	var hzLookupKey = types.NamespacedName{
		Name:      hazelcast.Name,
		Namespace: hazelcast.Namespace,
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
		By("Deleting Hazelcast", func() {
			Expect(k8sClient.Delete(ctx, hazelcast, client.PropagationPolicy(metav1.DeletePropagationForeground))).Should(Succeed())
		})

		By("Deleting Turbine", func() {
			Expect(k8sClient.Delete(ctx, turbineconfig.PingPong(hazelcast, ee), client.PropagationPolicy(metav1.DeletePropagationForeground))).Should(Succeed())
		})

		By("Deleting Ping Pong Deployments and Services", func() {
			svcList := v1.ServiceList{}
			Expect(k8sClient.List(ctx, &svcList, client.InNamespace(hzNamespace), client.MatchingLabels(testLabels))).Should(Succeed())
			for _, svc := range svcList.Items {
				Expect(k8sClient.Delete(ctx, &svc, client.PropagationPolicy(metav1.DeletePropagationForeground))).Should(Succeed())
			}

			deploymentList := appsv1.DeploymentList{}
			Expect(k8sClient.List(ctx, &deploymentList, client.InNamespace(hzNamespace), client.MatchingLabels(testLabels))).Should(Succeed())
			for _, d := range deploymentList.Items {
				Expect(k8sClient.Delete(ctx, &d, client.PropagationPolicy(metav1.DeletePropagationForeground))).Should(Succeed())
			}
		})

		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
	})

	createHz := func(h *hazelcastcomv1alpha1.Hazelcast) {
		By("Creating Hazelcast CR", func() {
			Expect(k8sClient.Create(context.Background(), h)).Should(Succeed())
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
			if t.Spec.Sidecar.Name == c.Name && strings.Contains(c.Image, t.Spec.Sidecar.Repository) {
				return &containers[i], nil
			}
		}
		return nil, errors.New("could not find the sidecar")
	}

	checkSidecar := func(d *appsv1.Deployment, t *hazelcastcomv1alpha1.Turbine, svc *v1.Service) {
		labels := d.Spec.Selector.MatchLabels
		list := v1.PodList{}
		Expect(k8sClient.List(context.Background(), &list, client.MatchingLabels(labels), client.InNamespace(d.Namespace))).Should(Succeed())
		for _, p := range list.Items {
			Expect(p.Spec.Containers).Should(HaveLen(2))
			s, err := getSidecar(p.Spec.Containers, t)
			Expect(err).Should(Succeed())
			Expect(s.Env).Should(Not(ContainElement(And(HaveField("Value", ""), HaveField("ValueFrom", nil)))))
			Expect(s.Env).Should(ContainElement(HaveField("Name", "CLUSTER_ADDRESS")))
			Expect(s.Env).Should(ContainElement(HaveField("Name", "APP_HTTP_PORT")))
			Expect(s.Env).Should(ContainElement(HaveField("Name", "TURBINE_POD_IP")))

			for _, env := range s.Env {
				if env.Name == "CLUSTER_ADDRESS" {
					clusterAddr := fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, svc.Spec.Ports[0].Port)
					By("checking CLUSTER_ADDRESS is equal to " + clusterAddr)
					Expect(env.Value).Should(Equal(clusterAddr))
				}
			}
		}

		Eventually(func() bool {
			depl := appsv1.Deployment{}
			key := types.NamespacedName{Name: d.Name, Namespace: d.Namespace}
			Expect(k8sClient.Get(context.Background(), key, &depl)).Should(Succeed())
			return depl.Status.ReadyReplicas == depl.Status.Replicas
		}, timeout, interval).Should(BeTrue())
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

	getHazelcastService := func(hz *hazelcastcomv1alpha1.Hazelcast) *v1.Service {
		By("getting Hazelcast service")
		srv := v1.Service{}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), types.NamespacedName{Name: hz.Name, Namespace: hz.Namespace}, &srv)
		}, timeout, interval).Should(Succeed())
		Expect(srv.Spec.ClusterIP).Should(Not(BeEmpty()))
		return &srv
	}

	Describe("Run Ping Pong", func() {
		It("should return Pong result", func() {
			createHz(hazelcast)

			turbine := turbineconfig.PingPong(hazelcast, ee)
			ping := turbineconfig.PingDeployment(hzNamespace, ee)
			pong := turbineconfig.PongDeployment(hzNamespace, ee)
			pingServ := turbineconfig.PingService(hzNamespace)
			pongServ := turbineconfig.PongService(hzNamespace)
			Expect(k8sClient.Create(context.Background(), turbine)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), ping)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), pingServ)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), pong)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), pongServ)).Should(Succeed())

			srv := getHazelcastService(hazelcast)
			checkSidecar(ping, turbine, srv)
			checkSidecar(pong, turbine, srv)

			externalAddr := waitForExternalAddress(pingServ)
			Eventually(func() string {
				resp, err := http.Get("http://" + externalAddr + "/do-ping-pong")
				if err != nil {
					return ""
				}
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return ""
				}
				return string(body)
			}, timeout, interval).Should(ContainSubstring("I sent ping and got back PONG!"))
		})
	})
})
