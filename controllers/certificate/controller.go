package certificate

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// certificateSecretName is the name of the secret which contains
	// the key and certificate. It will be mounted to the controller.
	// Make sure it is sync with the name in the configuration YAMLs.
	certificateSecretName = "webhook-server-cert"

	// serviceName is the name of the service which points to the controller's
	// webhook handler.
	serviceName = "webhook-service"

	// webhookConfigurationName is the name of the webhook configuration.
	webhookConfigurationName = "mutating-webhook-configuration"

	// webhookName is the name of the webhook which injects the turbine
	// sidecar to the pods.
	webhookName = "inject-turbine.hazelcast.com"

	// certFilesPath is the path where certificate and keys locate.
	certFilesPath = "/tmp/k8s-webhook-server/serving-certs"
)

type Reconciler struct {
	cli    client.Client
	reader client.Reader
	log    logr.Logger
	done   <-chan struct{}
	ns     string
}

func NewReconciler(client client.Client, reader client.Reader, logger logr.Logger, done <-chan struct{}, namespace string) *Reconciler {
	return &Reconciler{
		cli:    client,
		reader: reader,
		log:    logger,
		done:   done,
		ns:     namespace,
	}
}

func (c *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	if err := c.reconcile(ctx); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{Requeue: true, RequeueAfter: 1 * time.Hour}, nil
}

func (c *Reconciler) SetupWithManager(ctx context.Context, mgr controllerruntime.Manager) error {
	go c.cleanupWhenDone()

	err := c.reconcile(ctx)
	if err != nil {
		return err
	}

	if err := c.waitForLocalFiles(ctx); err != nil {
		return err
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}, builder.WithPredicates(NewNamespacedNameFilter(certificateSecretName, c.ns))).
		Watches(&source.Channel{Source: c.triggerPeriodic(time.Minute)}, &handler.EnqueueRequestForObject{}).
		Complete(c)
}

func (c *Reconciler) reconcile(ctx context.Context) error {
	secret, err := c.getOrCreateCertificateSecret(ctx)
	if err != nil {
		return err
	}

	update, err := c.updateRequired(secret)
	if err != nil {
		// Do not return, this is a soft error.
		c.log.Error(err, "Failed to check whether an update is required")
	}
	if !update {
		return nil
	}

	svc, err := c.getOrCreateWebhookService(ctx)
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

func (c *Reconciler) updateRequired(secret *corev1.Secret) (bool, error) {
	if secret.Type != corev1.SecretTypeTLS {
		return true, nil
	}
	if _, ok := secret.Data["ca.crt"]; !ok {
		return true, nil
	}
	if _, ok := secret.Data["tls.crt"]; !ok {
		return true, nil
	}
	if _, ok := secret.Data["tls.key"]; !ok {
		return true, nil
	}

	certData := secret.Data["tls.crt"]
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

func (c *Reconciler) waitForLocalFiles(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
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
	if err == nil && info.Size() > 0 {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	// This part should point to unexpected condition
	c.log.Info("Unexpected error", "file", path)
	return false
}

func (c *Reconciler) triggerPeriodic(duration time.Duration) <-chan event.GenericEvent {
	ch := make(chan event.GenericEvent)
	go func() {
		ticker := time.NewTicker(duration)
		for {
			select {
			case <-ticker.C:
				secret := corev1.Secret{}
				if err := c.reader.Get(
					context.Background(),
					client.ObjectKey{Name: certificateSecretName, Namespace: c.ns},
					&secret,
				); err != nil {
					c.log.Error(err, "periodic secret fetch failed")
					break
				}
				ch <- event.GenericEvent{Object: &secret}
			}
		}
	}()
	return ch
}

func (c *Reconciler) cleanupWhenDone() {
	<-c.done
	c.cleanup()
}

func (c *Reconciler) cleanup() {
	ctx := context.Background()
	if err := c.cli.Delete(ctx, defaultWebhookConfiguration(c.ns)); err != nil {
		c.log.Error(err, "Cleanup failed for webhook configuration")
	}
	if err := c.cli.Delete(ctx, defaultWebhookService(c.ns)); err != nil {
		c.log.Error(err, "Cleanup failed for webhook service")
	}
	if err := c.cli.Delete(ctx, defaultCertificateSecret(c.ns)); err != nil {
		c.log.Error(err, "Cleanup failed for certificate secret")
	}
}
