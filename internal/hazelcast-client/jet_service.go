package client

import (
	"context"
	"time"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

type JetService interface {
	RunJob(ctx context.Context, jobMetadata types.JobMetaData) error
	JobSummary(ctx context.Context, job *hazelcastv1alpha1.JetJob) (types.JobAndSqlSummary, error)
	JobSummaries(ctx context.Context) ([]types.JobAndSqlSummary, error)
	UpdateJobState(ctx context.Context, job types.JetTerminateJob) error
	ResumeJob(ctx context.Context, jobId int64) error
	ExportSnapshot(ctx context.Context, jobId int64, name string, cancelJob bool) (int64, error)
}

type HzJetService struct {
	client Client
}

func NewJetService(client Client) JetService {
	return &HzJetService{
		client: client,
	}
}

func (h HzJetService) ResumeJob(ctx context.Context, jobId int64) error {
	request := codec.EncodeJetResumeJobRequest(jobId)
	_, err := h.client.InvokeOnRandomTarget(ctx, request, nil)
	return err
}

func (h HzJetService) UpdateJobState(ctx context.Context, job types.JetTerminateJob) error {
	request := codec.EncodeJetTerminateJobRequest(job)
	_, err := h.client.InvokeOnRandomTarget(ctx, request, nil)
	return err
}

func (h HzJetService) RunJob(ctx context.Context, jobMetadata types.JobMetaData) error {
	request := codec.EncodeJetUploadJobMetaDataRequest(jobMetadata)
	_, err := h.client.InvokeOnRandomTarget(ctx, request, nil)
	return err
}

func (h HzJetService) JobSummaries(ctx context.Context) ([]types.JobAndSqlSummary, error) {
	request := codec.EncodeJetGetJobAndSqlSummaryListRequest()
	resp, err := h.client.InvokeOnRandomTarget(ctx, request, nil)
	if err != nil {
		return []types.JobAndSqlSummary{}, err
	}
	listResponse := codec.DecodeJetGetJobAndSqlSummaryListResponse(resp)
	return listResponse, nil
}

func (h HzJetService) JobSummary(ctx context.Context, job *hazelcastv1alpha1.JetJob) (types.JobAndSqlSummary, error) {
	listResponse, err := h.JobSummaries(ctx)
	if err != nil {
		return types.JobAndSqlSummary{}, err
	}
	for _, jobSummary := range listResponse {
		if jobSummary.NameOrId == job.JobName() {
			return jobSummary, nil
		}
	}
	return types.JobAndSqlSummary{}, nil
}

func (h HzJetService) ExportSnapshot(ctx context.Context, jobId int64, name string, cancelJob bool) (int64, error) {
	req := codec.EncodeJetExportSnapshotRequest(jobId, name, cancelJob)
	_, err := h.client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return 0, err
	}
	// Snapshot creation time can be taken from snapshot map '__jet.exportedSnapshot.<snapshot-name>'
	return time.Now().UnixMilli(), nil
}
