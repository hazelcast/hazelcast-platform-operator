package dialer

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/hazelcast/platform-operator-agent/sidecar"

	"github.com/hazelcast/hazelcast-platform-operator/internal/rest"
)

type Dialer struct {
	service *rest.DialerService
	config  *Config
}

type Config struct {
	MemberAddress string
	MTLSClient    *http.Client
}

func NewDialer(config *Config) (*Dialer, error) {
	host, _, err := net.SplitHostPort(config.MemberAddress)
	if err != nil {
		return nil, err
	}
	s, err := rest.NewDialerService("https://"+host+":8443", config.MTLSClient)
	if err != nil {
		return nil, err
	}

	return &Dialer{
		service: s,
		config:  config,
	}, nil
}

func (p *Dialer) TryDial(ctx context.Context, endpoints []string) error {
	dialResp, _, err := p.service.TryDial(ctx, &sidecar.DialRequest{
		Endpoints: endpoints,
	})
	if err != nil {
		return err
	}
	if !dialResp.Success {
		return errors.New(strings.Join(dialResp.ErrorMessages, "\t"))
	}
	return nil
}
