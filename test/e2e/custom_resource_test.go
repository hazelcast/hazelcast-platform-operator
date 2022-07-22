package e2e

import (
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/types"
)

var (
	labels            = map[string]string{}
	hzLookupKey       = types.NamespacedName{}
	mapLookupKey      = types.NamespacedName{}
	hbLookupKey       = types.NamespacedName{}
	hzSourceLookupKey = types.NamespacedName{}
	hzTargetLookupKey = types.NamespacedName{}
	wanLookupKey      = types.NamespacedName{}
	mcLookupKey       = types.NamespacedName{}
)

func setCRNamespace(ns string) {
	hzLookupKey.Namespace = ns
	mapLookupKey.Namespace = ns
	hbLookupKey.Namespace = ns
	hzSourceLookupKey.Namespace = ns
	hzTargetLookupKey.Namespace = ns
	wanLookupKey.Namespace = ns
	mcLookupKey.Namespace = ns
}

func setLabelAndCRName(n string) {
	n = n + "-" + randString(6)
	labels["test_suite"] = n
	hzLookupKey.Name = n
	mapLookupKey.Name = n
	hbLookupKey.Name = n
	hzSourceLookupKey.Name = "src" + "-" + n
	hzTargetLookupKey.Name = "trg" + "-" + n
	wanLookupKey.Name = n
	mcLookupKey.Name = n
	GinkgoWriter.Printf("Resource name is: %s\n", n)
	AddReportEntry("CR_ID:" + n)
}

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"0123456789"

func randString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
