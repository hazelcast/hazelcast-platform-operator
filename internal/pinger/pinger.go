package pinger

import (
	"context"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	"github.com/hazelcast/hazelcast-platform-operator/internal/rest"
	"net"
)

type Pinger struct {
	service *rest.PingerService
	config  *Config
}

type Config struct {
	MemberAddress string
	MTLSClient    *mtls.Client
}

func NewPinger(config *Config) (*Pinger, error) {
	host, _, err := net.SplitHostPort(config.MemberAddress)
	if err != nil {
		return nil, err
	}
	s, err := rest.NewPingerService("https://"+host+":8443", &config.MTLSClient.Client)
	if err != nil {
		return nil, err
	}

	return &Pinger{
		service: s,
		config:  config,
	}, nil
}

func (p *Pinger) Ping(ctx context.Context, endpoints string) (bool, error) {
	pingResp, _, err := p.service.Ping(ctx, &rest.PingRequest{
		Endpoints: endpoints,
	})
	if err != nil {
		return false, err
	}
	return pingResp.Success, err
}
