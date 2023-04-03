package client

import (
	"context"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

type JetService interface {
	RunJob(ctx context.Context, jobMetadata types.JobMetaData) error
	JobSummary(ctx context.Context, job *hazelcastv1alpha1.JetJob) (types.JobAndSqlSummary, error)
	JobSummaries(ctx context.Context) ([]types.JobAndSqlSummary, error)
}

type HzJetService struct {
	client Client
}

func NewJetService(client Client) JetService {
	return &HzJetService{
		client: client,
	}
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
