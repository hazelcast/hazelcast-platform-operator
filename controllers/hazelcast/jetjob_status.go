package hazelcast

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	client "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type jetJobStatusChecker struct {
	m               sync.RWMutex
	jobs            map[string]types.NamespacedName
	ticker          *time.Ticker
	checkInProgress bool
}

func newJetJobStatusChecker() *jetJobStatusChecker {
	return &jetJobStatusChecker{
		jobs:            make(map[string]types.NamespacedName),
		checkInProgress: false,
	}
}

type jetJobUpdateFunc func(context.Context, codecTypes.JobAndSqlSummary, types.NamespacedName)

func (sc *jetJobStatusChecker) storeJob(jobName string, nn types.NamespacedName) {
	sc.m.Lock()
	defer sc.m.Unlock()
	sc.jobs[jobName] = nn
}

func (sc *jetJobStatusChecker) deleteJob(jobName string) {
	sc.m.Lock()
	defer sc.m.Unlock()
	delete(sc.jobs, jobName)
}

func (sc *jetJobStatusChecker) loadJob(jobName string) (types.NamespacedName, bool) {
	sc.m.RLock()
	defer sc.m.RUnlock()
	name, ok := sc.jobs[jobName]
	return name, ok
}

func (sc *jetJobStatusChecker) runChecker(ctx context.Context, service client.JetService, jjUpdateF jetJobUpdateFunc, logger logr.Logger) {
	sc.m.Lock()
	if sc.checkInProgress {
		sc.m.Unlock()
		return
	}
	sc.checkInProgress = true
	sc.ticker = time.NewTicker(10 * time.Second)
	go func(f jetJobUpdateFunc) {
		for ; true; <-sc.ticker.C { //trick to make first tick happen immediately
			logger.V(util.DebugLevel).Info("Checking JetJob statuses by ticker...")
			sc.m.Lock()
			if len(sc.jobs) <= 0 {
				sc.checkInProgress = false
				sc.m.Unlock()
				sc.ticker.Stop()
				logger.V(util.DebugLevel).Info("No active JetJobs. Finishing status checks")
				return
			}
			sc.m.Unlock()
			summaries, err := service.JobSummaries(ctx)
			if err != nil {
				logger.Error(err, "Unable to fetch Jet Job summary")
				continue
			}
			for _, summary := range summaries {
				if jobNn, ok := sc.loadJob(summary.NameOrId); ok {
					f(ctx, summary, jobNn)
				}
			}
		}
	}(jjUpdateF)
	sc.m.Unlock()
}

type jetJobStatusBuilder struct {
	status hazelcastv1alpha1.JetJobStatusPhase
	err    error
	jobId  int64
}

func jetJobWithStatus(s hazelcastv1alpha1.JetJobStatusPhase) *jetJobStatusBuilder {
	return &jetJobStatusBuilder{
		status: s,
	}
}

func (jj *jetJobStatusBuilder) withJobId(jobId int64) *jetJobStatusBuilder {
	jj.jobId = jobId
	return jj
}

func failedJetJobStatus(err error) *jetJobStatusBuilder {
	return &jetJobStatusBuilder{
		status: hazelcastv1alpha1.JetJobFailed,
		err:    err,
	}
}
