package test

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func GetPodLogs(ctx context.Context, pod types.NamespacedName, podLogOptions *corev1.PodLogOptions) io.ReadCloser {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		panic(err)
	}
	// creates the clientset
	clientset := kubernetes.NewForConfigOrDie(config)
	p, err := clientset.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, v1.GetOptions{})
	if err != nil {
		panic(err)
	}
	if p.Status.Phase != corev1.PodFailed && p.Status.Phase != corev1.PodRunning {
		panic("Unable to get pod logs for the pod in Phase " + p.Status.Phase)
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, podLogOptions)
	podLogs, err := req.Stream(context.Background())
	if err != nil {
		panic(err)
	}
	return podLogs
}
