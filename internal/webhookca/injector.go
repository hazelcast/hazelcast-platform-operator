package webhookca

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func MaybeInject(mgr *manager.Manager, setupLog logr.Logger, namespace, deploymentName string) error {
	injected, err := maybeInjectCAForOLM(mgr)
	if err != nil {
		return err
	}

	if !injected {
		maybeInjectWebhook(mgr, setupLog, namespace, deploymentName)
		if err != nil {
			return err
		}
	}

	return nil
}
