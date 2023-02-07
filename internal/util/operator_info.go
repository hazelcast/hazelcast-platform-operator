package util

import (
	"context"
	"fmt"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type WatchedNsType string

const (
	WatchedNsTypeAll    WatchedNsType = "All"
	WatchedNsTypeMulti  WatchedNsType = "Multi"
	WatchedNsTypeSingle WatchedNsType = "Single"
	WatchedNsTypeOwn    WatchedNsType = "Own"
)

func WatchedNamespaceType() WatchedNsType {
	watchedNamespaces := WatchedNamespaces()
	operatorNamespace := OperatorNamespace()
	if operatorNamespace == "" {
		return WatchedNsTypeAll
	}

	switch {
	case len(watchedNamespaces) == 1 && (watchedNamespaces[0] == "" || watchedNamespaces[0] == "*"):
		return WatchedNsTypeAll
	case len(watchedNamespaces) == 1 && watchedNamespaces[0] == operatorNamespace:
		return WatchedNsTypeOwn
	case len(watchedNamespaces) == 1 && watchedNamespaces[0] != operatorNamespace:
		return WatchedNsTypeSingle
	case len(watchedNamespaces) > 1:
		return WatchedNsTypeMulti
	default:
		return WatchedNsTypeAll
	}

}

func WatchedNamespaces() []string {
	return strings.Split(os.Getenv(n.WatchedNamespacesEnv), ",")
}

func OperatorNamespace() string {
	return os.Getenv(n.NamespaceEnv)
}

func OperatorVersion() string {
	return os.Getenv(n.OperatorVersionEnv)
}

func PardotID() string {
	return os.Getenv(n.PardotIDEnv)
}

func OperatorID(c *rest.Config) types.UID {
	uid, err := operatorDeploymentID(c)
	if err == nil {
		return uid
	}

	return uuid.NewUUID()
}

func operatorDeploymentID(c *rest.Config) (types.UID, error) {
	ns := OperatorNamespace()
	podName := os.Getenv(n.PodNameEnv)

	if ns == "" || podName == "" {
		return "", fmt.Errorf("%s or %s is not defined", n.NamespaceEnv, n.PodNameEnv)
	}
	client, err := kubernetes.NewForConfig(c)
	if err != nil {
		return "", err
	}

	var d *appsv1.Deployment
	d, err = client.AppsV1().Deployments(ns).Get(context.TODO(), DeploymentName(podName), metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return d.UID, nil
}

func DeploymentName(podName string) string {
	s := strings.Split(podName, "-")
	return strings.Join(s[:len(s)-2], "-")
}
