package rest

import (
	"context"
	"net/http"
	"net/url"
)

type PingerService struct {
	client *Client
}

func NewPingerService(address string, c *http.Client) (*PingerService, error) {
	b, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	return &PingerService{client: &Client{
		BaseURL: b,
		client:  c,
	}}, nil
}

type PingRequest struct {
	Endpoints string `json:"endpoints"`
}

type PingResponse struct {
	Success bool `json:"success"`
}

func (p *PingerService) Ping(ctx context.Context, r *PingRequest) (*PingResponse, *http.Response, error) {
	u := "ping"

	uploadReq, err := p.client.NewRequest("POST", u, r)
	if err != nil {
		return nil, nil, err
	}

	var uploadResp *PingResponse
	httpResp, err := p.client.Do(ctx, uploadReq, uploadResp)
	if err != nil {
		return nil, httpResp, err
	}
	return uploadResp, httpResp, nil
}
