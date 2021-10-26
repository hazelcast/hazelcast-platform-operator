package e2e

import (
	"flag"
	"time"
)

var hzNamespace string
var interval time.Duration
var timeout time.Duration
var deleteTimeout time.Duration

func init() {
	flag.StringVar(&hzNamespace, "namespace", "default", "The namespace to run e2e tests")
	flag.DurationVar(&interval, "interval", 1*time.Second, "The length of time between checks")
	flag.DurationVar(&timeout, "timeout", 5*time.Minute, "Timeout for test steps")
	flag.DurationVar(&deleteTimeout, "delete-timeout", 5*time.Minute, "Timeout for resource deletions")
}
