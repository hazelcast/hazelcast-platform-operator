package integration

import (
	"flag"
	"time"
)

var (
	ee       bool
	timeout  time.Duration
	interval time.Duration
)

func init() {
	flag.BoolVar(&ee, "ee", false, "Flag to define whether Enterprise edition of Hazelcast will be used")
	flag.DurationVar(&interval, "interval", time.Millisecond*250, "The length of time between checks")
	flag.DurationVar(&timeout, "eventually-timeout", time.Second*10, "Timeout for test steps")
}
