package rest

import (
	"context"
	"net/http"
	"net/url"
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

type LocalBackups struct {
	Backups []string `json:"backups"`
}

type LocalBackupsOptions struct {
	BackupBaseDir string `json:"backup_base_dir"`
	MemberID      int    `json:"member_id"`
}

func (s *LocalBackupService) LocalBackups(ctx context.Context, opts *LocalBackupsOptions) (*LocalBackups, *http.Response, error) {
	u := "backup"

	req, err := s.client.NewRequest("GET", u, opts)
	if err != nil {
		return nil, nil, err
	}

	localBackups := new(LocalBackups)
	resp, err := s.client.Do(ctx, req, localBackups)
	if err != nil {
		return nil, resp, err
	}

	return localBackups, resp, nil
}
