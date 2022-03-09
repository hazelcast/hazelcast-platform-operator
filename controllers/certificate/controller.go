package certificate

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-logr/logr"
	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// webhookName is the name of the webhook which injects the turbine
	// sidecar to the pods.
	webhookName = "inject-turbine.hazelcast.com"

	// certFilesPath is the path where certificate and keys locate.
	certFilesPath = "/tmp/k8s-webhook-server/serving-certs"
)

type Reconciler struct {
	cli        client.Client
	reader     client.Reader
	log        logr.Logger
	name       string
	ns         string
	namePrefix string
}

func NewReconciler(client client.Client, reader client.Reader, logger logr.Logger, name, namespace string) *Reconciler {
	prefix, err := inferNamePrefix(name)
	if err != nil {
		panic(err)
	}
	return &Reconciler{
		cli:        client,
		reader:     reader,
		log:        logger,
		name:       name,
		ns:         namespace,
		namePrefix: prefix,
	}
}

func (c *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	c.log.Info("Certificate reconcile started", "Request", request.String())
	if err := c.reconcile(ctx); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{Requeue: true, RequeueAfter: 1 * time.Hour}, nil
}

func (c *Reconciler) SetupWithManager(ctx context.Context, mgr controllerruntime.Manager) error {
	err := c.reconcile(ctx)
	if err != nil {
		return err
	}

	if err := c.waitForLocalFiles(ctx); err != nil {
		return err
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}, builder.WithPredicates(NewNamespacedNameFilter(certificateSecretName(c.namePrefix), c.ns))).
		Complete(c)
}

func (c *Reconciler) reconcile(ctx context.Context) error {
	c.log.Info("Reconciling certificate")

	secret, err := c.getCertificateSecret(ctx)
	if err != nil {
		return err
	}

	webhook, err := c.getWebhookConfiguration(ctx)
	if err != nil {
		return err
	}

	update, err := c.updateRequired(secret, webhook)
	if err != nil {
		// Do not return, this is a soft error.
		c.log.Error(err, "Failed to check whether an update is required")
	}
	if !update {
		return nil
	}
	defer func() {
		go c.triggerPodUpdate()
	}()

	svc, err := c.getWebhookService(ctx)
	if err != nil {
		return err
	}

	bundle, err := c.updateCertificateSecret(ctx, secret, svc)
	if err != nil {
		return err
	}

	if err := c.updateWebhook(ctx, bundle); err != nil {
		return err
	}
	return nil
}

// updateRequired checks whether the certificate renewal is required.
// It checks wheter the certificate is valid for the current period, and
// it is consistent with the client configuration in the webhook configuration
func (c *Reconciler) updateRequired(secret *corev1.Secret, webhook *admv1.MutatingWebhookConfiguration) (bool, error) {
	if secret.Type != corev1.SecretTypeTLS {
		return true, nil
	}
	if val, ok := secret.Data["ca.crt"]; !ok || len(val) == 0 {
		return true, nil
	}
	if val, ok := secret.Data["tls.crt"]; !ok || len(val) == 0 {
		return true, nil
	}
	if val, ok := secret.Data["tls.key"]; !ok || len(val) == 0 {
		return true, nil
	}

	// Check equality between secret data and webhook data
	certData := secret.Data["tls.crt"]
	caBundle := getCABundle(webhook)
	if !bytes.Equal(certData, caBundle) {
		return true, nil
	}

	// Checking for certificate validity, if it is not valid then update.
	b, _ := pem.Decode(certData)
	if b == nil {
		return true, fmt.Errorf("invalid encoding for PEM")
	}
	if b.Type != "CERTIFICATE" {
		return true, fmt.Errorf("invalid type in PEM block: %s", b.Type)
	}
	cert, err := x509.ParseCertificate(b.Bytes)
	if err != nil {
		return true, err
	}

	if time.Now().Add(24 * time.Hour).After(cert.NotAfter) {
		return true, nil
	} else {
		return false, nil
	}
}

// waitForLocalFiles blocks until the new file contents are mounted to the controller
// pod after the data in the secret is updated.
func (c *Reconciler) waitForLocalFiles(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if c.localFilesExist() {
				return nil
			} else {
				c.log.Info("Waiting for certificate files")
			}
		}
	}
}

func (c *Reconciler) localFilesExist() bool {
	files := []string{"ca.crt", "tls.crt", "tls.key"}
	for _, file := range files {
		if !c.fileExistsAndNotEmpty(path.Join(certFilesPath, file)) {
			return false
		}
	}
	return true
}

func (c *Reconciler) fileExistsAndNotEmpty(path string) bool {
	info, err := os.Stat(path)
	if err == nil {
		if info.Size() > 0 {
			return true
		} else {
			c.log.Info("File is empty", "file", path)
			return false
		}
	}
	if errors.Is(err, os.ErrNotExist) {
		c.log.Info("File does not exist", "file", path)
		return false
	}
	// This part should point to unexpected condition
	c.log.Info("Unexpected error", "file", path)
	return false
}

func inferNamePrefix(podName string) (string, error) {
	i := strings.Index(podName, "controller-manager")
	if i == -1 {
		return "", fmt.Errorf("cannot infer name prefix: unexpected name for controller pod")
	}
	return podName[:i], nil
}

func getCABundle(webhook *admv1.MutatingWebhookConfiguration) []byte {
	for _, wh := range webhook.Webhooks {
		if wh.Name == webhookName {
			return wh.ClientConfig.CABundle
		}
	}
	return []byte{}
}
