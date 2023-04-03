package hazelcast

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

func Test_jetJobCheckerConcurrency(t *testing.T) {
	wg := sync.WaitGroup{}
	testDoneWg := &sync.WaitGroup{}
	js := &fakeJetService{
		jobs: []codecTypes.JobAndSqlSummary{
			{
				NameOrId: "job1",
			},
		},
	}
	checker := newJetJobStatusChecker()
	storeNewJob(checker, testDoneWg, "job1")
	wg.Add(1)

	var firstJobs []string
	updateF := func(_ context.Context, sum codecTypes.JobAndSqlSummary, nn types.NamespacedName) {
		firstJobs = append(firstJobs, nn.Name)
		wg.Wait()
		testDoneWg.Done()
	}
	checker.runChecker(context.Background(), js, updateF, logr.Logger{})

	anotherUpdateF := func(_ context.Context, sum codecTypes.JobAndSqlSummary, nn types.NamespacedName) {
		testDoneWg.Add(1)
		t.Errorf("the second job should not run!")
	}

	js.jobs = append(js.jobs, codecTypes.JobAndSqlSummary{NameOrId: "job2"}, codecTypes.JobAndSqlSummary{NameOrId: "job3"})
	storeNewJob(checker, testDoneWg, "job2")
	checker.runChecker(context.Background(), js, anotherUpdateF, logr.Logger{})
	storeNewJob(checker, testDoneWg, "job3")
	wg.Done()
	testDoneWg.Wait()

	if !reflect.DeepEqual(firstJobs, []string{"job1", "job2", "job3"}) {
		t.Errorf("the first running function did not update all three jobs")
	}
}

func Test_jetJobStatusCheckerDeadLock(t *testing.T) {
	jsc := newJetJobStatusChecker()
	jobName := "test-job"
	nsName := types.NamespacedName{Name: "test-namespace"}
	jsc.storeJob(jobName, nsName)

	var wg sync.WaitGroup
	wg.Add(2)
	f := func(_ context.Context, summary codecTypes.JobAndSqlSummary, nn types.NamespacedName) {}
	go func() {
		jsc.runChecker(context.Background(), &fakeJetService{}, f, logr.Logger{})
		wg.Done()
	}()
	go func() {
		jsc.runChecker(context.Background(), &fakeJetService{}, f, logr.Logger{})
		wg.Done()
	}()
	wg.Wait()
	// If the test doesn't fail or hang indefinitely, it means that the deadlock issue didn't occur
}

func storeNewJob(checker *jetJobStatusChecker, wg *sync.WaitGroup, jobName string) {
	wg.Add(1)
	checker.storeJob(jobName, types.NamespacedName{Name: jobName, Namespace: "default"})
}

type fakeJetService struct {
	jobs []codecTypes.JobAndSqlSummary
}

func (f fakeJetService) RunJob(_ context.Context, _ codecTypes.JobMetaData) error {
	return nil
}

func (f fakeJetService) JobSummary(ctx context.Context, job *hazelcastv1alpha1.JetJob) (codecTypes.JobAndSqlSummary, error) {
	summaries, err := f.JobSummaries(ctx)
	for _, summary := range summaries {
		if summary.NameOrId == job.JobName() {
			return summary, nil
		}
	}
	return codecTypes.JobAndSqlSummary{}, err
}

func (f fakeJetService) JobSummaries(_ context.Context) ([]codecTypes.JobAndSqlSummary, error) {
	return f.jobs, nil
}
