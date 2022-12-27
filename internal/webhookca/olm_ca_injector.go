package webhookca

import (
	"errors"
	"os"
	"path/filepath"

	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// For more info, read the following:
// https://olm.operatorframework.io/docs/advanced-tasks/adding-admission-and-conversion-webhooks/#certificate-authority-requirements
// https://github.com/sruthakeerthikotla/memcachedwebhook/blob/main/webhookWithOLM.md

const (
	webhookServerPathForOLM = "/tmp/k8s-webhook-server/serving-certs/"
)

func injectCAForOLM(mgr *manager.Manager) (bool, error) {
	certPathForOLM := filepath.Join(webhookServerPathForOLM, core.TLSCertKey)
	if _, err := os.Stat(certPathForOLM); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// if the OLM generated certificate does not exist, it is not an error, it means it is not the OLM case
			return false, nil
		}
		return false, err
	}

	srv := (*mgr).GetWebhookServer()
	srv.CertDir = webhookServerPathForOLM
	srv.CertName = core.TLSCertKey
	srv.KeyName = core.TLSPrivateKeyKey

	return true, nil
}
