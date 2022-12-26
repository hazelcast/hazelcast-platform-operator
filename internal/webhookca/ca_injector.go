package webhookca

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	admissionregistration "k8s.io/api/admissionregistration/v1"
	core "k8s.io/api/core/v1"
)

const (
	certManagerAnnotation = "cert-manager.io/inject-ca-from"
	webhookServerPath     = "/tmp/k8s-webhook-server/serving-certs"
)

func maybeInjectWebhook(mgr *manager.Manager, setupLog logr.Logger, namespace, deploymentName string) error {
	webhookName := types.NamespacedName{
		Name:      strings.ReplaceAll(deploymentName, "controller-manager", "validating-webhook-configuration"),
		Namespace: namespace,
	}

	serviceName := types.NamespacedName{
		Name:      strings.ReplaceAll(deploymentName, "controller-manager", "webhook-service"),
		Namespace: namespace, // service namespace is also hardcoded in webhook manifest
	}

	setupLog.Info("Starting CA injector", "webhook", webhookName, "service", serviceName)

	webhookCAInjector, err := NewCAInjector((*mgr).GetClient(), webhookName, serviceName)
	if err != nil {
		setupLog.Error(err, "unable to create webhook ca injector")
		return err
	}

	if err := (*mgr).Add(webhookCAInjector); err != nil {
		setupLog.Error(err, "unable to run webhook ca injector")
		return err
	}

	return nil
}

type CAInjector struct {
	kubeClient  client.Client
	webhookName types.NamespacedName
	tlsCert     []byte
}

func NewCAInjector(kubeClient client.Client, webhookName, serviceName types.NamespacedName) (*CAInjector, error) {
	if webhookName.Namespace == "" {
		webhookName.Namespace = "default"
	}

	if serviceName.Namespace == "" {
		serviceName.Namespace = "default"
	}

	c := CAInjector{
		kubeClient:  kubeClient,
		webhookName: webhookName,
	}

	certPath := filepath.Join(webhookServerPath, core.TLSCertKey)
	if _, err := os.Stat(certPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}

		// tls.crt not found, we need to create new
		ca, err := generateCA(serviceName.Name, serviceName.Namespace)
		if err != nil {
			return nil, err
		}

		// ensure directory exists
		if err := os.MkdirAll(webhookServerPath, 0755); err != nil {
			return nil, err
		}

		if err := os.WriteFile(certPath, ca[core.TLSCertKey], 0600); err != nil {
			return nil, err
		}

		keyPath := filepath.Join(webhookServerPath, core.TLSPrivateKeyKey)
		if err := os.WriteFile(keyPath, ca[core.TLSPrivateKeyKey], 0600); err != nil {
			return nil, err
		}

		c.tlsCert = ca[core.TLSCertKey]
		return &c, nil
	}

	// we want to keep ValidatingWebhookConfiguration caBundle in sync with tls.crt on disk
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	c.tlsCert = data
	return &c, nil
}

func (c *CAInjector) Start(ctx context.Context) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		config := admissionregistration.ValidatingWebhookConfiguration{}
		if err := c.kubeClient.Get(ctx, c.webhookName, &config); err != nil {
			return err
		}

		// skip if configuration is using cert-manager
		if _, ok := config.Annotations[certManagerAnnotation]; ok {
			return nil
		}

		// update CA in all webhooks
		scope := admissionregistration.NamespacedScope
		for i := range config.Webhooks {
			config.Webhooks[i].ClientConfig.CABundle = c.tlsCert
			for j := range config.Webhooks[i].Rules {
				config.Webhooks[i].Rules[j].Scope = &scope
			}
		}

		return c.kubeClient.Update(ctx, &config)
	})
}
