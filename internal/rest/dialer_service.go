package rest

import (
	"context"
	"net/http"
	"net/url"
)

type DialerService struct {
	client *Client
}

func NewDialerService(address string, c *http.Client) (*DialerService, error) {
	b, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	return &DialerService{client: &Client{
		BaseURL: b,
		client:  c,
	}}, nil
}

type DialRequest struct {
	Endpoints string `json:"endpoints"`
}

type DialResponse struct {
	Success bool `json:"success"`
}

func (p *DialerService) TryDial(ctx context.Context, r *DialRequest) (*DialResponse, *http.Response, error) {
	u := "dial"

	dialReq, err := p.client.NewRequest("POST", u, r)
	if err != nil {
		return nil, nil, err
	}

	dialResp := new(DialResponse)
	httpResp, err := p.client.Do(ctx, dialReq, dialResp)
	if err != nil {
		return nil, httpResp, err
	}
	return dialResp, httpResp, nil
}
