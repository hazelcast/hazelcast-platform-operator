package integration

import (
	"context"
	v1 "k8s.io/api/core/v1"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func assertDoesNotExist(name types.NamespacedName, obj client.Object) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), name, obj)
		if err == nil {
			return false
		}
		return errors.IsNotFound(err)
	}, timeout, interval).Should(BeTrue())
}

func assertExists(name types.NamespacedName, obj client.Object) {
	Eventually(func() error {
		return k8sClient.Get(context.Background(), name, obj)
	}, timeout, interval).Should(Succeed())
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

func lookupKey(cr metav1.Object) types.NamespacedName {
	return types.NamespacedName{
		Name:      cr.GetName(),
		Namespace: cr.GetNamespace(),
	}
}

func getStatefulSet(cr metav1.Object) *appsv1.StatefulSet {
	sts := &appsv1.StatefulSet{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), lookupKey(cr), sts)
	}, timeout, interval).Should(Succeed())

	return sts
}

func getConfigMap(cr metav1.Object) *v1.ConfigMap {
	cfg := &v1.ConfigMap{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), lookupKey(cr), cfg)
	}, timeout, interval).Should(Succeed())

	return cfg
}
