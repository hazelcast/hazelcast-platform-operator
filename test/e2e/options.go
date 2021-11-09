package e2e

import (
	"flag"
	"time"
)

var (
	hzNamespace   string
	interval      time.Duration
	timeout       time.Duration
	deleteTimeout time.Duration
	ee            bool
)

func init() {
	flag.StringVar(&hzNamespace, "namespace", "default", "The namespace to run e2e tests")
	flag.DurationVar(&interval, "interval", 1*time.Second, "The length of time between checks")
	flag.DurationVar(&timeout, "timeout", 5*time.Minute, "Timeout for test steps")
	flag.DurationVar(&deleteTimeout, "delete-timeout", 5*time.Minute, "Timeout for resource deletions")
	flag.BoolVar(&ee, "ee", true, "Flag to define whether Enterprise edition of Hazelcast will be used")
}
