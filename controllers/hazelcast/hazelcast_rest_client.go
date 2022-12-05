package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
)

// Section contains the REST API endpoints.
const (
	changeState = "/hazelcast/rest/management/cluster/changeState"
	getState    = "/hazelcast/rest/management/cluster/state"
	forceStart  = "/hazelcast/rest/management/cluster/forceStart"
	hotBackup   = "/hazelcast/rest/management/cluster/hotBackup"
)

type ClusterState string

const (
	// Active is the default cluster state. Cluster continues to operate without restrictions.
	Active ClusterState = "ACTIVE"

	// Passive is the state when the partition table is frozen and partition assignments are not performed.
	Passive ClusterState = "PASSIVE"
)

type RestClient struct {
	url         string
	clusterName string
}

type stateResponse struct {
	State string `json:"state"`
}

func NewRestClient(h *v1alpha1.Hazelcast) *RestClient {
	return &RestClient{
		url:         hzclient.RestUrl(h),
		clusterName: h.Spec.ClusterName,
	}
}

func (c *RestClient) ForceStart(ctx context.Context) error {
	d := fmt.Sprintf("%s&", c.clusterName)
	ctxT, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := postRequest(ctxT, d, c.url, forceStart)
	if err != nil {
		return err
	}
	res, err := c.executeRequest(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, res.Body)
		return fmt.Errorf("unexpected HTTP error: %s, %s", res.Status, buf.String())
	}
	return nil
}

func (c *RestClient) GetState(ctx context.Context) (string, error) {
	d := fmt.Sprintf("%s&", c.clusterName)
	ctxT, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := postRequest(ctxT, d, c.url, getState)
	if err != nil {
		return "", err
	}
	res, err := c.executeRequest(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code when checking for Cluster state: %d, %s",
			res.StatusCode, res.Status)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	s := &stateResponse{}
	err = json.Unmarshal(b, s)
	if err != nil {
		return "", err
	}
	return s.State, nil
}

func (c *RestClient) ChangeState(ctx context.Context, state ClusterState) error {
	d := fmt.Sprintf("%s&&%s", c.clusterName, state)
	ctxT, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := postRequest(ctxT, d, c.url, changeState)
	if err != nil {
		return err
	}
	res, err := c.executeRequest(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var rBody map[string]string
	err = json.NewDecoder(res.Body).Decode(&rBody)
	if err != nil {
		return err
	}
	if s := rBody["status"]; s != "success" {
		return fmt.Errorf("unexpected Hot Backup status: %s", s)
	}
	return nil
}

func (c *RestClient) HotBackup(ctx context.Context) error {
	d := fmt.Sprintf("%s&", c.clusterName)
	ctxT, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := postRequest(ctxT, d, c.url, hotBackup)
	if err != nil {
		return err
	}
	res, err := c.executeRequest(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

func (c *RestClient) executeRequest(req *http.Request) (*http.Response, error) {
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return res, err
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, res.Body)
		return res, fmt.Errorf("unexpected HTTP error: %s, %s", res.Status, buf.String())
	}
	return res, nil
}

func postRequest(ctx context.Context, data string, url string, endpoint string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url+endpoint, strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	return req, nil
}
