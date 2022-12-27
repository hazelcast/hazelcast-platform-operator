package webhookca

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func Inject(mgr *manager.Manager, setupLog logr.Logger, namespace, deploymentName string) error {
	injected, err := injectCAForOLM(mgr)
	if err != nil {
		return err
	}

	if !injected {
		err = injectWebhook(mgr, setupLog, namespace, deploymentName)
		if err != nil {
			return err
		}
	}

	return nil
}
