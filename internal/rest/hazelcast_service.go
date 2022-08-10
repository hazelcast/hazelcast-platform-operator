package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type HazelcastService struct {
	client *Client
}

func NewHazelcastService(address string) (*HazelcastService, error) {
	baseURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	return &HazelcastService{
		client: &Client{
			BaseURL: baseURL,
			client:  &http.Client{},
		},
	}, nil
}

type HazelcastState struct {
	Status string `json:"status,omitempty"`
	State  string `json:"state,omitempty"`

	// exceptionResponse
	Message string `json:"message,omitempty"`
}

func (s *HazelcastService) GetState(ctx context.Context, clusterName string) (*HazelcastState, *http.Response, error) {
	u := "hazelcast/rest/management/cluster/state"
	b := fmt.Sprintf("%s&", clusterName) // "${CLUSTERNAME}&${PASSWORD}"

	req, err := s.client.NewRequest("POST", u, b)
	if err != nil {
		return nil, nil, err
	}

	state := new(HazelcastState)
	resp, err := s.client.Do(ctx, req, state)
	if err != nil {
		return nil, resp, err
	}

	if err := checkState(resp, state); err != nil {
		return nil, resp, err
	}

	return state, resp, nil
}

func (s *HazelcastService) ChangeState(ctx context.Context, clusterName, newState string) (*HazelcastState, *http.Response, error) {
	u := "hazelcast/rest/management/cluster/changeState"
	b := fmt.Sprintf("%s&&%s", clusterName, newState) // "${CLUSTERNAME}&${PASSWORD}&${STATE}"

	req, err := s.client.NewRequest("POST", u, b)
	if err != nil {
		return nil, nil, err
	}

	state := new(HazelcastState)
	resp, err := s.client.Do(ctx, req, state)
	if err != nil {
		return nil, resp, err
	}

	if err := checkState(resp, state); err != nil {
		return nil, resp, err
	}

	return state, resp, nil
}

type HazelcastStatus struct {
	Status string `json:"status,omitempty"`

	// exceptionResponse
	Message string `json:"message,omitempty"`
}

func (s *HazelcastService) HotBackup(ctx context.Context, clusterName string) (*http.Response, error) {
	u := "hazelcast/rest/management/cluster/hotBackup"
	b := fmt.Sprintf("%s&", clusterName) // "${CLUSTERNAME}&${PASSWORD}"

	req, err := s.client.NewRequest("POST", u, b)
	if err != nil {
		return nil, err
	}

	status := new(HazelcastStatus)
	resp, err := s.client.Do(ctx, req, status)
	if err != nil {
		return resp, err
	}

	if err := checkStatus(resp, status); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *HazelcastService) HotBackupInterrupt(ctx context.Context, clusterName string) (*http.Response, error) {
	u := "hazelcast/rest/management/cluster/hotBackupInterrupt"
	b := fmt.Sprintf("%s&", clusterName) // "${CLUSTERNAME}&${PASSWORD}"

	req, err := s.client.NewRequest("POST", u, b)
	if err != nil {
		return nil, err
	}

	status := new(HazelcastStatus)
	resp, err := s.client.Do(ctx, req, status)
	if err != nil {
		return resp, err
	}

	if err := checkStatus(resp, status); err != nil {
		return resp, err
	}

	return resp, nil
}

type HazelcastError struct {
	Status   string
	Message  string
	Response *http.Response
}

func (r *HazelcastError) Error() string {
	return fmt.Sprintf("%v %v: %d %v %+v",
		r.Response.Request.Method, r.Response.Request.URL, r.Response.StatusCode, r.Status, r.Message)
}

func checkState(r *http.Response, s *HazelcastState) error {
	if s.Status == "success" {
		return nil
	}
	return &HazelcastError{Status: s.Status, Message: s.Message, Response: r}
}

func checkStatus(r *http.Response, s *HazelcastStatus) error {
	if s.Status == "success" {
		return nil
	}
	return &HazelcastError{Status: s.Status, Message: s.Message, Response: r}
}
