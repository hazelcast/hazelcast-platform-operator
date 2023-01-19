package rest

import (
	"context"
	"net/http"
	"net/url"

	"github.com/hazelcast/platform-operator-agent/sidecar"
)

type LocalBackupService struct {
	client *Client
}

func NewLocalBackupService(address string, httpClient *http.Client) (*LocalBackupService, error) {
	baseURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	return &LocalBackupService{
		client: &Client{
			BaseURL: baseURL,
			client:  httpClient,
		},
	}, nil
}

func (s *LocalBackupService) LocalBackups(ctx context.Context, opts *sidecar.Req) (*sidecar.Resp, *http.Response, error) {
	u := "backup"

	req, err := s.client.NewRequest("GET", u, opts)
	if err != nil {
		return nil, nil, err
	}

	localBackups := new(sidecar.Resp)
	resp, err := s.client.Do(ctx, req, localBackups)
	if err != nil {
		return nil, resp, err
	}

	return localBackups, resp, nil
}
