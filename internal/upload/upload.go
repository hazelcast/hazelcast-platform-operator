package upload

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	"github.com/hazelcast/hazelcast-platform-operator/internal/rest"
	"github.com/hazelcast/platform-operator-agent/backup"
)

var (
	errUploadNotStarted     = errors.New("upload not started")
	errUploadAlreadyStarted = errors.New("upload already started")
)

type Upload struct {
	service  *rest.UploadService
	uploadID *uuid.UUID
	config   *Config
}

type Config struct {
	MemberAddress string
	BucketURI     string
	BackupBaseDir string
	HazelcastName string
	SecretName    string
	MTLSClient    *mtls.Client
	MemberID      int
}

func NewUpload(config *Config) (*Upload, error) {
	host, _, err := net.SplitHostPort(config.MemberAddress)
	if err != nil {
		return nil, err
	}
	s, err := rest.NewUploadService("https://"+host+":8443", &config.MTLSClient.Client)
	if err != nil {
		return nil, err
	}
	return &Upload{
		service: s,
		config:  config,
	}, nil
}

func (u *Upload) Start(ctx context.Context) error {
	if u.uploadID != nil {
		return errUploadAlreadyStarted
	}
	upload, _, err := u.service.Upload(ctx, &backup.UploadReq{
		BucketURL:       u.config.BucketURI,
		BackupBaseDir:   u.config.BackupBaseDir,
		HazelcastCRName: u.config.HazelcastName,
		SecretName:      u.config.SecretName,
		MemberID:        u.config.MemberID,
	})
	if err != nil {
		return err
	}

	u.uploadID = &upload.ID
	return nil
}

func (u *Upload) Wait(ctx context.Context) (string, error) {
	if u.uploadID == nil {
		return "", errUploadNotStarted
	}
	for {
		status, _, err := u.service.Status(ctx, *u.uploadID)
		if err != nil {
			return "", err
		}

		switch status.Status {
		case "CANCELED":
			return "", fmt.Errorf("Upload canceled: %s", status.Message)
		case "FAILURE":
			return "", fmt.Errorf("Upload failed: %s", status.Message)
		case "SUCCESS":
			return status.BackupKey, nil
		case "IN_PROGRESS":
			// expected, check status again (no return)
		default:
			return "", errors.New("Upload unknown status: " + status.Status)
		}

		// wait for timer or context to cancel
		select {
		case <-time.After(1 * time.Second):
			continue
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func (u *Upload) Cancel(ctx context.Context) error {
	if u.uploadID == nil {
		return errUploadNotStarted
	}
	_, err := u.service.Delete(ctx, *u.uploadID)
	return err
}
