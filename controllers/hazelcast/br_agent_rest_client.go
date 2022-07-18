package hazelcast

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

// Section contains the REST API endpoints.
const (
	uploadBackup = "/upload"
)

type uploadRequest struct {
	BucketURL        string `json:"bucket_url"`
	BackupFolderPath string `json:"backup_folder_path"`
	HazelcastCRName  string `json:"hz_cr_name"`
	SecretName       string `json:"secret_name"`
	MemberUUID       string `json:"member_uuid"`
}

type AgentRestClient struct {
	h                *v1alpha1.Hazelcast
	bucketURL        string
	backupFolderPath string
	hazelcastCRName  string
	secretName       string
}

func NewAgentRestClient(h *v1alpha1.Hazelcast, hb *v1alpha1.HotBackup) *AgentRestClient {
	return &AgentRestClient{
		h:                h,
		bucketURL:        hb.Spec.BucketURI,
		backupFolderPath: h.Spec.Persistence.BaseDir,
		hazelcastCRName:  hb.Spec.HazelcastResourceName,
		secretName:       hb.Spec.Secret,
	}
}

func (ac *AgentRestClient) UploadBackup(ctx context.Context) error {
	ctxT, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	for _, member := range ac.h.Status.Members {
		address := fmt.Sprintf("%s:%d", member.Ip, n.DefaultAgentPort)
		upload := uploadRequest{
			BucketURL:        ac.bucketURL,
			BackupFolderPath: ac.backupFolderPath + "/hot-backup",
			HazelcastCRName:  ac.hazelcastCRName,
			SecretName:       ac.secretName,
			MemberUUID:       member.Uid,
		}
		reqBody, err := json.Marshal(upload)
		if err != nil {
			return err
		}
		req, err := postRequestWithBody(ctxT, reqBody, address, uploadBackup)
		if err != nil {
			return fmt.Errorf("request creation failed: %s, address --> %q , URL --> %q ", err, address, address+uploadBackup)
		}
		res, err := ac.executeRequest(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
	}
	return nil
}

func (ac *AgentRestClient) executeRequest(req *http.Request) (*http.Response, error) {
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

func postRequestWithBody(ctx context.Context, body []byte, address string, endpoint string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://%s%s", address, endpoint), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	return req, nil
}
