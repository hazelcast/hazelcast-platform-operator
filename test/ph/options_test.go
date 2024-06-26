package ph

import (
	"flag"
	"os"
	"time"
)

var (
	hzNamespace   string
	interval      time.Duration
	timeout       time.Duration
	deleteTimeout time.Duration
	version       string
)

func init() {
	flag.StringVar(&hzNamespace, "namespace", "default", "The namespace to run phone home tests")
	flag.DurationVar(&interval, "interval", 1*time.Second, "The length of time between checks")
	flag.DurationVar(&timeout, "eventually-timeout", 5*time.Minute, "Timeout for test steps")
	flag.DurationVar(&deleteTimeout, "delete-timeout", 5*time.Minute, "Timeout for resource deletions")
	flag.StringVar(&version, "VERSION", os.Getenv("VERSION"), "Image version")
}
