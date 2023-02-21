package test

import (
	"bufio"
	"fmt"
	"io"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
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
	if *spec.ClusterSize != matcher.Expected.ClusterSize {
		return false, fmt.Errorf(
			"expected ClusterSize is %d but actual is %d", matcher.Expected.ClusterSize, *spec.ClusterSize)
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

func EventuallyInLogs(lr *LogReader, intervals ...interface{}) AsyncAssertion {
	return Eventually(func() string {
		return lr.Read()
	}, intervals...)
}

func EventuallyInLogsUnordered(lr *LogReader, intervals ...interface{}) AsyncAssertion {
	return Eventually(func() []string {
		lr.Read()
		return lr.History
	}, intervals...)
}

type LogReader struct {
	reader  io.ReadCloser
	lines   chan string
	History []string
}

func NewLogReader(r io.ReadCloser) *LogReader {
	lr := &LogReader{
		reader:  r,
		lines:   make(chan string),
		History: make([]string, 0),
	}
	go startLogReader(lr)
	return lr
}

// Read returns the new line of the logs in a non-blocking fashion.
// It polls the next line and returns it.
// If the next line doesn't exist, it returns empty string.
func (lr *LogReader) Read() string {
	select {
	case l := <-lr.lines:
		lr.History = append(lr.History, l)
		return l
	default:
		return ""
	}
}

func (lr *LogReader) Close() error {
	return lr.reader.Close()
}

func startLogReader(lr *LogReader) {
	s := bufio.NewScanner(lr.reader)
	for s.Scan() {
		lr.lines <- s.Text()
	}
	close(lr.lines)
}
