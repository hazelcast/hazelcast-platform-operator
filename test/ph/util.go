package ph

import (
	"context"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

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

func googleCloudProjectName() string {
	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		return "hazelcast-33"
	}
	return projectID
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
