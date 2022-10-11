package e2e

import (
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/types"
)

var (
	labels         = map[string]string{}
	hzLookupKey    = types.NamespacedName{}
	mapLookupKey   = types.NamespacedName{}
	wanLookupKey   = types.NamespacedName{}
	mcLookupKey    = types.NamespacedName{}
	hbLookupKey    = types.NamespacedName{}
	mmLookupKey    = types.NamespacedName{}
	topicLookupKey = types.NamespacedName{}
)

var (
	hzSrcLookupKey  = types.NamespacedName{}
	hzTrgLookupKey  = types.NamespacedName{}
	sourceLookupKey = types.NamespacedName{}
	targetLookupKey = types.NamespacedName{}
)

func setCRNamespace(ns string) {
	hzLookupKey.Namespace = ns
	mapLookupKey.Namespace = ns
	hbLookupKey.Namespace = ns
	mcLookupKey.Namespace = ns
	wanLookupKey.Namespace = ns
	topicLookupKey.Namespace = ns
	hzSrcLookupKey.Namespace = ns
	hzTrgLookupKey.Namespace = ns
	sourceLookupKey.Namespace = sourceNamespace
	targetLookupKey.Namespace = targetNamespace
	mmLookupKey.Namespace = ns
}

func setLabelAndCRName(n string) {
	n = n + "-" + randString(6)
	By(fmt.Sprintf("setting the label and CR with name '%s'", n))
	labels["test_suite"] = n
	hzLookupKey.Name = n
	wanLookupKey.Name = n
	mapLookupKey.Name = n
	hbLookupKey.Name = n
	mcLookupKey.Name = n
	topicLookupKey.Name = n
	hzSrcLookupKey.Name = "src-" + n
	hzTrgLookupKey.Name = "trg-" + n
	sourceLookupKey.Name = "src-" + n
	targetLookupKey.Name = "trg-" + n
	mmLookupKey.Name = n
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
