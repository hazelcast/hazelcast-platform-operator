package localbackup

import (
	"context"
	"errors"
	"net"

	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	"github.com/hazelcast/hazelcast-platform-operator/internal/rest"
)

type LocalBackup struct {
	service *rest.LocalBackupService
	config  *Config
}

type Config struct {
	MemberAddress string
	BackupBaseDir string
	MTLSClient    *mtls.Client
	MemberID      int
}

func NewLocalBackup(config *Config) (*LocalBackup, error) {
	host, _, err := net.SplitHostPort(config.MemberAddress)
	if err != nil {
		return nil, err
	}
	s, err := rest.NewLocalBackupService("https://"+host+":8443", &config.MTLSClient.Client)
	if err != nil {
		return nil, err
	}
	return &LocalBackup{
		service: s,
		config:  config,
	}, nil
}

func (u *LocalBackup) GetLatestLocalBackup(ctx context.Context) (string, error) {
	localBackups, _, err := u.service.LocalBackups(ctx, &rest.LocalBackupsOptions{
		BackupBaseDir: u.config.BackupBaseDir,
		MemberID:      u.config.MemberID,
	})
	if err != nil {
		return "", err
	}

	if len(localBackups.Backups) == 0 {
		return "", errors.New("There are no local backups")
	}

	return localBackups.Backups[len(localBackups.Backups)-1], nil

}
