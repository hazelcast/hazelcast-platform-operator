package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/types"
	"math/rand"
)

var (
	labels             = map[string]string{}
	hzLookupKey        = types.NamespacedName{}
	hzSourceLookupKey  = types.NamespacedName{}
	hzTargetLookupKey  = types.NamespacedName{}
	hzSrcLookupKey     = types.NamespacedName{}
	hzTrgLookupKey     = types.NamespacedName{}
	mapLookupKey       = types.NamespacedName{}
	mapSourceLookupKey = types.NamespacedName{}
	mapTargetLookupKey = types.NamespacedName{}
	wanLookupKey       = types.NamespacedName{}
	wanSourceLookupKey = types.NamespacedName{}
	wanTargetLookupKey = types.NamespacedName{}
	mcLookupKey        = types.NamespacedName{}
	hbLookupKey        = types.NamespacedName{}
)

func setCRNamespace(ns string) {
	hzLookupKey.Namespace = ns
	mapLookupKey.Namespace = ns
	hbLookupKey.Namespace = ns
	mcLookupKey.Namespace = ns
	wanLookupKey.Namespace = ns
	hzSrcLookupKey.Namespace = ns
	hzTrgLookupKey.Namespace = ns
	mapSourceLookupKey.Namespace = "src-ns"
	mapTargetLookupKey.Namespace = "trg-ns"
	hzSourceLookupKey.Namespace = "src-ns"
	hzTargetLookupKey.Namespace = "trg-ns"
	wanSourceLookupKey.Namespace = "src-ns"
	wanTargetLookupKey.Namespace = "trg-ns"
}

func setLabelAndCRName(n string) {
	n = n + "-" + randString(6)
	labels["test_suite"] = n
	hzLookupKey.Name = n
	wanLookupKey.Name = n
	mapLookupKey.Name = n
	hbLookupKey.Name = n
	mcLookupKey.Name = n
	mapSourceLookupKey.Name = "src-" + n
	mapTargetLookupKey.Name = "trg-" + n
	hzSourceLookupKey.Name = "src-" + n
	hzTargetLookupKey.Name = "trg-" + n
	hzSrcLookupKey.Name = "src-" + n
	hzTrgLookupKey.Name = "trg-" + n
	wanSourceLookupKey.Name = "src-" + n
	wanTargetLookupKey.Name = "trg-" + n
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
