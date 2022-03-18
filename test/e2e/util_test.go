package e2e

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hazelcast/hazelcast-platform-operator/test"
)

func useExistingCluster() bool {
	return strings.ToLower(os.Getenv("USE_EXISTING_CLUSTER")) == "true"
}

func runningLocally() bool {
	return strings.ToLower(os.Getenv("RUN_MANAGER_LOCALLY")) == "true"
}

func controllerManagerName() string {
	np := os.Getenv("NAME_PREFIX")
	if np == "" {
		return "hazelcast-platform-controller-manager"
	}

	return np + "controller-manager"
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
func assertDoesNotExist(name types.NamespacedName, obj client.Object) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), name, obj)
		if err == nil {
			return false
		}
		return errors.IsNotFound(err)
	}, deleteTimeout, interval).Should(BeTrue())
}

func assertExists(name types.NamespacedName, obj client.Object) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), name, obj)
		return err == nil
	}, timeout, interval).Should(BeTrue())
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

func cleanUpHostPath(namespace, hostPath, hzDir string) {
	name := "cleanup-hostpath"
	lb := map[string]string{
		"cleanup": "hazelcast-hostPath",
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: lb,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lb,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:    "cleanup",
						Image:   "ubuntu",
						Command: []string{"sh"},
						Args:    []string{"-c", fmt.Sprintf("rm -rf /host/%s || echo 'Could not delete'; echo 'done'; sleep 120", hzDir)},
						SecurityContext: &v1.SecurityContext{
							RunAsNonRoot:             &[]bool{false}[0],
							RunAsUser:                &[]int64{0}[0],
							Privileged:               &[]bool{true}[0],
							ReadOnlyRootFilesystem:   &[]bool{false}[0],
							AllowPrivilegeEscalation: &[]bool{true}[0],
							Capabilities: &v1.Capabilities{
								Drop: []v1.Capability{"ALL"},
							},
						},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "hostpath-hazelcast",
								MountPath: "/host",
							},
						},
					}},
					TerminationGracePeriodSeconds: &[]int64{0}[0],
					Volumes: []v1.Volume{
						{
							Name: "hostpath-hazelcast",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: hostPath,
									Type: &[]v1.HostPathType{v1.HostPathDirectory}[0],
								},
							},
						},
					},
				},
			}},
	}
	Expect(k8sClient.Create(context.Background(), ds)).Should(Succeed())
	pods := waitForDSPods(ds, lb)
	for _, pod := range pods.Items {
		running := assertRunningOrFailedMount(pod, hostPath)
		if !running {
			continue
		}

		logs := test.GetPodLogs(context.Background(), types.NamespacedName{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		}, &corev1.PodLogOptions{
			Follow: true,
		})
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, timeout, logInterval).
			Should(ContainSubstring("done"))

	}
	Expect(k8sClient.Delete(context.Background(), ds)).Should(Succeed())
}

func waitForDSPods(ds *appsv1.DaemonSet, lb client.MatchingLabels) *corev1.PodList {
	pods := &corev1.PodList{}
	podLabels := lb
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, ds)
		if err != nil {
			return false
		}
		return ds.Status.DesiredNumberScheduled != 0
	}, timeout, interval).Should(BeTrue())

	Eventually(func() bool {
		err := k8sClient.List(context.Background(), pods, client.InNamespace(ds.Namespace), podLabels)
		if err != nil {
			return false
		}
		return int(ds.Status.DesiredNumberScheduled) == len(pods.Items)
	}, timeout, interval).Should(BeTrue())
	return pods
}

func assertRunningOrFailedMount(p v1.Pod, hostPath string) bool {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		panic(err)
	}
	clientset := kubernetes.NewForConfigOrDie(config)

	running := false
	Eventually(func() bool {
		ev := metav1.ListOptions{FieldSelector: fmt.Sprintf("involvedObject.name=%s", p.Name), TypeMeta: metav1.TypeMeta{Kind: "Pod"}}
		events, err := clientset.CoreV1().Events(p.Namespace).List(context.TODO(), ev)
		if err != nil {
			return false
		}
		for _, event := range events.Items {
			if event.Message == fmt.Sprintf("MountVolume.SetUp failed for volume \"hostpath-hazelcast\" : hostPath type check failed: %s is not a directory", hostPath) {
				return true
			}
		}
		pod := &v1.Pod{}
		err = k8sClient.Get(context.Background(), types.NamespacedName{Name: p.Name, Namespace: p.Namespace}, pod)
		if err != nil {
			return false
		}
		if pod.Status.Phase == v1.PodRunning {
			running = true
			return true
		}
		return false
	}, timeout, interval).Should(BeTrue())

	return running
}
