package rest

import (
	"context"
	"net/http"
	"net/url"

	"github.com/hazelcast/platform-operator-agent/sidecar"
)

type FileDownloadService struct {
	client *Client
}

func NewFileDownloadService(address string, httpClient *http.Client) (*FileDownloadService, error) {
	baseURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	return &FileDownloadService{
		client: &Client{
			BaseURL: baseURL,
			client:  httpClient,
		},
	}, nil
}

func (s *FileDownloadService) Download(ctx context.Context, opts sidecar.DownloadFileReq) (*http.Response, error) {
	u := "download"
	req, err := s.client.NewRequest("POST", u, opts)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(ctx, req, nil)
	return resp, err
}
