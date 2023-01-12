package e2e

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	. "time"

	. "github.com/onsi/ginkgo/v2"
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

func GetControllerManagerName() string {
	return os.Getenv("DEPLOYMENT_NAME")
}

func GetSuiteName() string {
	if !ee {
		return "Operator Suite OS"
	}
	return "Operator Suite EE"
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
	}, 8*Minute, interval).Should(BeTrue())
}

func assertExists(name types.NamespacedName, obj client.Object) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), name, obj)
		return err == nil
	}, 20*Second, interval).Should(BeTrue())
}

func cleanUpHostPath(namespace, hostPath, hzDir string) {
	By(fmt.Sprintf("cleanup hostpath '%s' and directory '%s' for namespace '%s'", hostPath, hzDir, namespace), func() {
		lb := map[string]string{
			"cleanup": "hazelcast-hostPath",
		}
		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "hostpath-",
				Namespace:    namespace,
				Labels:       lb,
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
							Name:  "cleanup",
							Image: "busybox",

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
			test.EventuallyInLogs(scanner, 40*Second, logInterval).
				Should(ContainSubstring("done"))
		}
		DeleteAllOf(ds, nil, namespace, lb)
	})
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
	}, 1*Minute, interval).Should(BeTrue())
	Eventually(func() bool {
		err := k8sClient.List(context.Background(), pods, client.InNamespace(ds.Namespace), podLabels)
		if err != nil {
			return false
		}
		return int(ds.Status.DesiredNumberScheduled) == len(pods.Items)
	}, 2*Minute, interval).Should(BeTrue())
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
	}, 1*Minute, interval).Should(BeTrue())
	return running
}

func deletePVCs(lk types.NamespacedName) {
	pvcL := &corev1.PersistentVolumeClaimList{}
	Eventually(func() bool {
		err := k8sClient.List(context.Background(), pvcL, client.InNamespace(lk.Namespace))
		if err != nil {
			return false
		}
		for _, pvc := range pvcL.Items {
			if strings.Contains(pvc.Name, lk.Name) {
				err = k8sClient.Delete(context.Background(), &pvc, client.PropagationPolicy(metav1.DeletePropagationForeground))
				if err != nil {
					return false
				}
			}
		}
		return true
	}, 1*Minute, interval).Should(BeTrue())
}

func deletePods(lk types.NamespacedName) {
	By("deleting pods", func() {
		// Because pods get recreated by the StatefulSet controller, we are not using the eventually block here
		podL := &corev1.PodList{}
		err := k8sClient.List(context.Background(), podL, client.InNamespace(lk.Namespace))
		Expect(err).To(BeNil())
		for _, pod := range podL.Items {
			if strings.Contains(pod.Name, lk.Name) {
				err = k8sClient.Delete(context.Background(), &pod)
				Expect(err).To(BeNil())
			}
		}
	})
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
	if objList == nil {
		return
	}

	objListVal := reflect.ValueOf(objList)

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

	}, 10*Minute, interval).Should(Equal(int(0)))
}
