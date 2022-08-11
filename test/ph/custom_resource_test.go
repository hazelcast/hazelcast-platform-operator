package ph

import (
	"k8s.io/apimachinery/pkg/types"
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
)

var (
	labels      = map[string]string{}
	hzLookupKey = types.NamespacedName{}
	mcLookupKey = types.NamespacedName{}
)

func setCRNamespace(ns string) {
	hzLookupKey.Namespace = ns
	mcLookupKey.Namespace = ns
}

func setLabelAndCRName(n string) {
	n = n + "-" + randString(6)
	labels["test_suite"] = n
	hzLookupKey.Name = n
	mcLookupKey.Name = n
	GinkgoWriter.Printf("Resource name is: %s\n", n)
	AddReportEntry("CR_ID:" + n)
}

const charset = "abcdefghijklmnopqrstuvwxyz" + "0123456789"

func randString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
