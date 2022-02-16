package hazelcast

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
)

// Section contains the REST API endpoints.
const (
	changeState = "/hazelcast/rest/management/cluster/changeState"
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
	httpClient  *http.Client
}

func NewRestClient(h *v1alpha1.Hazelcast) *RestClient {
	return &RestClient{
		url:         fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", h.Name, h.Namespace, n.DefaultHzPort),
		clusterName: h.Spec.ClusterName,
		httpClient: &http.Client{
			Timeout: time.Minute,
		},
	}
}

func (c *RestClient) ChangeState(state ClusterState) error {
	d := fmt.Sprintf("%s&&%s", c.clusterName, state)
	req, err := postRequest(d, c.url, changeState)
	if err != nil {
		return err
	}
	res, err := c.executeRequest(req)
	if err != nil {
		return err
	}
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

func (c *RestClient) HotBackup() error {
	d := fmt.Sprintf("%s&", c.clusterName)
	req, err := postRequest(d, c.url, hotBackup)
	if err != nil {
		return err
	}
	_, err = c.executeRequest(req)
	return err
}

func (c *RestClient) executeRequest(req *http.Request) (*http.Response, error) {
	res, err := c.httpClient.Do(req)
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

func postRequest(data string, url string, endpoint string) (*http.Request, error) {
	req, err := http.NewRequest("POST", url+endpoint, strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	return req, nil
}
