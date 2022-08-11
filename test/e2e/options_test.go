package e2e

import (
	"flag"
	"math/rand"
	"os"
	"time"
)

var (
	hzNamespace        string
	mapSourceNamespace string
	mapTargetNamespace string
	hzSourceNamespace  string
	hzTargetNamespace  string
	wanSourceNamespace string
	wanTargetNamespace string
	context1           string
	context2           string
	interval           time.Duration
	ee                 bool
)

func init() {
	flag.StringVar(&hzNamespace, "namespace", "default", "The namespace to run e2e tests")
	flag.StringVar(&mapSourceNamespace, "mapSourceNamespace", os.Getenv("mapSourceNamespace"), "The map source namespace to run e2e wan tests")
	flag.StringVar(&mapTargetNamespace, "mapTargetNamespace", os.Getenv("mapTargetNamespace"), "The map target namespace to run e2e wan tests")
	flag.StringVar(&hzSourceNamespace, "hzSourceNamespace", os.Getenv("hzSourceNamespace"), "The hazelcast source namespace to run e2e wan tests")
	flag.StringVar(&hzTargetNamespace, "hzTargetNamespace", os.Getenv("hzTargetNamespace"), "The hazelcast target namespace to run e2e wan tests")
	flag.StringVar(&wanSourceNamespace, "wanSourceNamespace", os.Getenv("wanSourceNamespace"), "The wan source namespace to run e2e wan tests")
	flag.StringVar(&wanTargetNamespace, "wanTargetNamespace", os.Getenv("wanTargetNamespace"), "The wan target namespace to run e2e wan tests")
	flag.StringVar(&context1, "FIRST_CONTEXT_NAME", os.Getenv("FIRST_CONTEXT_NAME"), "First context name")
	flag.StringVar(&context2, "SECOND_CONTEXT_NAME", os.Getenv("SECOND_CONTEXT_NAME"), "Second context name")
	flag.DurationVar(&interval, "interval", 1*time.Second, "The length of time between checks")
	flag.BoolVar(&ee, "ee", true, "Flag to define whether Enterprise edition of Hazelcast will be used")
	rand.Seed(time.Now().UnixNano())
}
