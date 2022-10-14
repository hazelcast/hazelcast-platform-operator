package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
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

type Upload struct {
	ID uuid.UUID `json:"ID,omitempty"`
}

type UploadOptions struct {
	BucketURL       string `json:"bucket_url"`
	BackupBaseDir   string `json:"backup_base_dir"`
	HazelcastCRName string `json:"hz_cr_name"`
	SecretName      string `json:"secret_name"`
	MemberID        int    `json:"member_id"`
}

func (s *UploadService) Upload(ctx context.Context, opts *UploadOptions) (*Upload, *http.Response, error) {
	u := "upload"

	req, err := s.client.NewRequest("POST", u, opts)
	if err != nil {
		return nil, nil, err
	}

	upload := new(Upload)
	resp, err := s.client.Do(ctx, req, upload)
	if err != nil {
		return nil, resp, err
	}

	return upload, resp, nil
}

type UploadStatus struct {
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
	BackupKey string `json:"backup_key,omitempty"`
}

func (s *UploadService) Status(ctx context.Context, uploadID uuid.UUID) (*UploadStatus, *http.Response, error) {
	u := fmt.Sprintf("upload/%v", uploadID)

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	status := new(UploadStatus)
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
