package test

import (
	"fmt"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

func EqualSpecs(expected *HazelcastSpecValues, ee bool) types.GomegaMatcher {
	return &HazelcastSpecEqual{
		Expected: expected,
		ee:       ee,
	}
}

type HazelcastSpecEqual struct {
	Expected *HazelcastSpecValues
	ee       bool
}

func (matcher HazelcastSpecEqual) Match(actual interface{}) (success bool, err error) {
	spec, ok := actual.(*hazelcastv1alpha1.HazelcastSpec)
	if !ok {
		return false, fmt.Errorf("type of %v should be &hazelcastv1alpha1.HazelcastSpec", actual)
	}
	if spec.ClusterSize != matcher.Expected.ClusterSize {
		return false, fmt.Errorf(
			"expected ClusterSize is %d but actual is %d", matcher.Expected.ClusterSize, spec.ClusterSize)
	}
	if spec.Repository != matcher.Expected.Repository {
		return false, fmt.Errorf(
			"expected Repository is %s but actual is %s", matcher.Expected.Repository, spec.Repository)
	}
	if spec.Version != matcher.Expected.Version {
		return false, fmt.Errorf(
			"expected Version is %s but actual is %s", matcher.Expected.Version, spec.Version)
	}
	if spec.ImagePullPolicy != matcher.Expected.ImagePullPolicy {
		return false, fmt.Errorf(
			"expected ImagePullPolicy is %s but actual is %s", matcher.Expected.ImagePullPolicy, spec.ImagePullPolicy)
	}
	if matcher.ee && spec.LicenseKeySecret != matcher.Expected.LicenseKey {
		return false, fmt.Errorf(
			"expected LicenseKeySecret is %s but actual is %s", matcher.Expected.LicenseKey, spec.LicenseKeySecret)
	}
	return true, nil
}

func (matcher HazelcastSpecEqual) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to equal", matcher.Expected)
}

func (matcher HazelcastSpecEqual) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to equal", matcher.Expected)
}
