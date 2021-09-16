package e2e

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	util "github.com/hazelcast/hazelcast-enterprise-operator/controllers/util"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func useExistingCluster() bool {
	return strings.ToLower(os.Getenv("USE_EXISTING_CLUSTER")) == "true"
}

func decodeYAML(r io.Reader, obj interface{}) error {
	decoder := yaml.NewYAMLToJSONDecoder(r)
	return decoder.Decode(obj)
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

func waitDeletionOf(name types.NamespacedName, obj client.Object) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), name, obj)
		if err == nil {
			return false
		}
		return errors.IsNotFound(err)
	}, deleteTimeout, interval).Should(BeTrue())
}

func waitCreationOf(name types.NamespacedName, obj client.Object) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), name, obj)
		return err == nil
	}, timeout, interval).Should(BeTrue())
}

func waitForStsReady(name types.NamespacedName, sts *appsv1.StatefulSet, expectedReplicas int32) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), name, sts)
		if err != nil {
			return false
		}
		return util.IsStatefulSetReady(sts, expectedReplicas)
	}, timeout, interval).Should(BeTrue())
}

func deleteIfExists(name types.NamespacedName, obj client.Object) error {
	err := k8sClient.Get(context.Background(), name, obj)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	err = k8sClient.Delete(context.Background(), obj)
	return err
}

func loadFromFile(obj interface{}, fileName string) error {
	f, err := os.Open(fmt.Sprintf("../../config/samples/%s", fileName))
	if err != nil {
		return err
	}
	defer f.Close()

	return decodeYAML(f, obj)
}
