package types

import (
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type TopicConfigs struct {
	Topics []TopicConfig `xml:"topic"`
}

type TopicConfig struct {
	Name                  string `xml:"name,attr"`
	GlobalOrderingEnabled bool   `xml:"global-ordering-enabled"`
	MultiThreadingEnabled bool   `xml:"multi-threading-enabled"`
	StatisticsEnabled     bool   `xml:"statistics-enabled"`
	//nullable
	ListenerConfigs []ListenerConfigHolder
}

// Default values are explicitly written for all fields that are not nullable
// even though most are the same with the default values in Go.
func DefaultTopicConfigInput() *TopicConfig {
	return &TopicConfig{
		GlobalOrderingEnabled: n.DefaultTopicGlobalOrderingEnabled,
		MultiThreadingEnabled: n.DefaultTopicMultiThreadingEnabled,
		StatisticsEnabled:     n.DefaultTopicStatisticsEnabled,
	}
}
