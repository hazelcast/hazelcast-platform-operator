package integration

import (
	"flag"
)

var (
	ee bool
)

func init() {
	flag.BoolVar(&ee, "ee", false, "Flag to define whether Enterprise edition of Hazelcast will be used")
}
