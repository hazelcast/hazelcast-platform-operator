package localbackup

import (
	"context"
	"errors"
	"net"

	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	"github.com/hazelcast/hazelcast-platform-operator/internal/rest"
	"github.com/hazelcast/platform-operator-agent/sidecar"
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
	s, err := rest.NewLocalBackupService(hzclient.AgentUrl(host), &config.MTLSClient.Client)
	if err != nil {
		return nil, err
	}
	return &LocalBackup{
		service: s,
		config:  config,
	}, nil
}

func (u *LocalBackup) GetLatestLocalBackup(ctx context.Context) (string, error) {
	localBackups, _, err := u.service.LocalBackups(ctx, &sidecar.Req{
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
