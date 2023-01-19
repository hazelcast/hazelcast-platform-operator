package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/hazelcast/platform-operator-agent/sidecar"
)

type UploadService struct {
	client *Client
}

func NewUploadService(address string, httpClient *http.Client) (*UploadService, error) {
	baseURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	return &UploadService{
		client: &Client{
			BaseURL: baseURL,
			client:  httpClient,
		},
	}, nil
}

func (s *UploadService) Upload(ctx context.Context, opts *sidecar.UploadReq) (*sidecar.UploadResp, *http.Response, error) {
	u := "upload"

	req, err := s.client.NewRequest("POST", u, opts)
	if err != nil {
		return nil, nil, err
	}

	upload := new(sidecar.UploadResp)
	resp, err := s.client.Do(ctx, req, upload)
	if err != nil {
		return nil, resp, err
	}

	return upload, resp, nil
}

func (s *UploadService) Status(ctx context.Context, uploadID uuid.UUID) (*sidecar.StatusResp, *http.Response, error) {
	u := fmt.Sprintf("upload/%v", uploadID)

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	status := new(sidecar.StatusResp)
	resp, err := s.client.Do(ctx, req, status)
	if err != nil {
		return nil, resp, err
	}

	return status, resp, nil
}

func (s *UploadService) Delete(ctx context.Context, uploadID uuid.UUID) (*http.Response, error) {
	u := fmt.Sprintf("upload/%v", uploadID)

	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(ctx, req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}
