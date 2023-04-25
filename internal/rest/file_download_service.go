package rest

import (
	"context"
	"net/http"
	"net/url"
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

// TODO reuse sidecar
type DownloadFileReq struct {
	URL        string `json:"url"`
	FileName   string `json:"file_name"`
	DestDir    string `json:"dest_dir"`
	SecretName string `json:"secret_name"`
}

func (s *FileDownloadService) Download(ctx context.Context, opts DownloadFileReq) (*http.Response, error) {
	u := "download"
	req, err := s.client.NewRequest("POST", u, opts)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(ctx, req, nil)
	return resp, err
}
