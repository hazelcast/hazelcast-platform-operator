package e2e

import (
	"flag"
	"math/rand"
	"os"
	"time"
)

var (
	hzNamespace     string
	sourceNamespace string
	targetNamespace string
	context1        string
	context2        string
	interval        time.Duration
	ee              bool
)

func init() {
	flag.StringVar(&hzNamespace, "namespace", "default", "The namespace to run e2e tests")
	flag.StringVar(&sourceNamespace, "sourceNamespace", os.Getenv("sourceNamespace"), "The source namespace to run e2e wan tests")
	flag.StringVar(&targetNamespace, "targetNamespace", os.Getenv("targetNamespace"), "The target namespace to run e2e wan tests")
	flag.StringVar(&context1, "FIRST_CONTEXT_NAME", os.Getenv("FIRST_CONTEXT_NAME"), "First context name")
	flag.StringVar(&context2, "SECOND_CONTEXT_NAME", os.Getenv("SECOND_CONTEXT_NAME"), "Second context name")
	flag.DurationVar(&interval, "interval", 100*time.Millisecond, "The length of time between checks")
	flag.BoolVar(&ee, "ee", true, "Flag to define whether Enterprise edition of Hazelcast will be used")
	rand.Seed(time.Now().UnixNano())
}
