package rest

import (
	"context"
	"net/http"
	"net/url"
)

type BundleService struct {
	client *Client
}

type BundleReq struct {
	URL        string `json:"url"`
	SecretName string `json:"secret_name"`
	DestDir    string `json:"dest_dir"`
}

func NewBundleService(address string, httpClient *http.Client) (*BundleService, error) {
	baseURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	return &BundleService{
		client: &Client{
			BaseURL: baseURL,
			client:  httpClient,
		},
	}, nil
}

func (s *BundleService) Download(ctx context.Context, opts BundleReq) (*http.Response, error) {
	u := "bundle"
	req, err := s.client.NewRequest("POST", u, opts)
	if err != nil {
		return nil, err
	}
	return s.client.Do(ctx, req, nil)
}
