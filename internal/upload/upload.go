package upload

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/hazelcast/hazelcast-platform-operator/internal/rest"
)

var (
	errUploadNotStarted     = errors.New("Upload not started")
	errUploadAlreadyStarted = errors.New("Upload already started")
	errUploadCanceled       = errors.New("Upload canceled")
	errUploadFailed         = errors.New("Upload failed")
)

type Upload struct {
	service  *rest.UploadService
	uploadID *uuid.UUID
	config   *Config
}

type Config struct {
	MemberAddress string
	BucketURI     string
	BackupPath    string
	HazelcastName string
	SecretName    string
}

func NewUpload(config *Config) (*Upload, error) {
	host, _, err := net.SplitHostPort(config.MemberAddress)
	if err != nil {
		return nil, err
	}
	s, err := rest.NewUploadService("http://" + host + ":8080")
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
	upload, _, err := u.service.Upload(ctx, &rest.UploadOptions{
		BucketURL:        u.config.BucketURI,
		BackupFolderPath: u.config.BackupPath,
		HazelcastCRName:  u.config.HazelcastName,
		SecretName:       u.config.SecretName,
	})
	if err != nil {
		return err
	}

	u.uploadID = &upload.ID
	return nil
}

func (u *Upload) Wait(ctx context.Context) error {
	if u.uploadID == nil {
		return errUploadNotStarted
	}
	for {
		status, _, err := u.service.Status(ctx, *u.uploadID)
		if err != nil {
			return err
		}

		switch status.Status {
		case "CANCELED":
			return errUploadCanceled
		case "FAILURE":
			return errUploadFailed
		case "SUCCESS":
			return nil
		case "IN_PROGRESS":
			// expected, check status again (no return)
		default:
			return errors.New("Upload unknown status: " + status.Status)
		}

		// wait for timer or context to cancel
		select {
		case <-time.After(1 * time.Second):
			continue
		case <-ctx.Done():
			return ctx.Err()
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
