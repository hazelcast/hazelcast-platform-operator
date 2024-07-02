package integration

import (
	"flag"
	"time"
)

var (
	timeout  time.Duration
	interval time.Duration
)

func init() {
	flag.DurationVar(&interval, "interval", time.Millisecond*250, "The length of time between checks")
	flag.DurationVar(&timeout, "eventually-timeout", time.Second*10, "Timeout for test steps")
}
