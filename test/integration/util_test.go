package integration

import (
	"context"
	"fmt"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	v1 "k8s.io/api/core/v1"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
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

func assertExistsAndBeAsExpected[o client.Object](name types.NamespacedName, obj o, predicate func(o) bool) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), name, obj)
		if err != nil {
			return false
		}
		return predicate(obj)
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

func deleteResource(name types.NamespacedName, obj client.Object) {
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), name, obj)
		if err != nil {
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

func getSecret(cr metav1.Object) *v1.Secret {
	s := &v1.Secret{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), lookupKey(cr), s)
	}, timeout, interval).Should(Succeed())

	return s
}

func randomObjectMeta(ns string, annotations ...string) metav1.ObjectMeta {
	var annotationMap map[string]string
	if l := len(annotations); l > 0 {
		annotationMap = make(map[string]string)
		for i, str := range annotations {
			if i%2 == 0 {
				annotationMap[str] = ""
			} else {
				annotationMap[annotations[i-1]] = str
			}
		}
	}
	return metav1.ObjectMeta{
		Name:        fmt.Sprintf("%s", uuid.NewUUID()),
		Namespace:   ns,
		Annotations: annotationMap,
	}
}

func defaultHazelcastSpecValues() *test.HazelcastSpecValues {
	licenseKey := ""
	repository := n.HazelcastRepo
	if ee {
		licenseKey = n.LicenseKeySecret
		repository = n.HazelcastEERepo
	}
	return &test.HazelcastSpecValues{
		ClusterSize:     n.DefaultClusterSize,
		Repository:      repository,
		Version:         n.HazelcastVersion,
		LicenseKey:      licenseKey,
		ImagePullPolicy: n.HazelcastImagePullPolicy,
	}
}
