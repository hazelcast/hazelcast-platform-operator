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
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	ns     string
}

func NewReconciler(client client.Client, reader client.Reader, logger logr.Logger, namespace string) *Reconciler {
	return &Reconciler{
		cli:    client,
		reader: reader,
		log:    logger,
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

func (c *Reconciler) getOrCreateCertificateSecret(ctx context.Context) (*corev1.Secret, error) {
	secret, err := c.getCertificateSecret(ctx)
	if err == nil {
		return secret, nil
	} else if kerrors.IsNotFound(err) {
		return c.createCertificateSecret(ctx)
	}
	return nil, fmt.Errorf("failed to get-or-create certificate secret: %w", err)
}

func (c *Reconciler) getCertificateSecret(ctx context.Context) (*corev1.Secret, error) {
	secret := corev1.Secret{}
	err := c.reader.Get(ctx, client.ObjectKey{Name: certificateSecretName, Namespace: c.ns}, &secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate secret: %w", err)
	}
	return &secret, nil
}

func (c *Reconciler) createCertificateSecret(ctx context.Context) (*corev1.Secret, error) {
	err := c.cli.Create(ctx, defaultCertificateSecret(c.ns))
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create certificate secret: %w", err)
	}
	return c.getCertificateSecret(ctx)
}

func (c *Reconciler) updateCertificateSecret(
	ctx context.Context,
	secret *corev1.Secret,
	service *corev1.Service,
) (*Bundle, error) {
	bundle, err := createSelfSigned(service)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate bundle: %w", err)
	}

	certEnc := encodeCertificateFromBundle(bundle)
	keyEnc := encodeKeyFromBundle(bundle)
	s := secret.DeepCopy()
	s.Data = map[string][]byte{
		"ca.crt":  certEnc,
		"tls.crt": certEnc,
		"tls.key": keyEnc,
	}

	if err := c.cli.Update(ctx, s); err != nil {
		return nil, fmt.Errorf("failed to update certificate secret: %w", err)
	}
	return bundle, nil
}

func (c *Reconciler) getOrCreateWebhookService(ctx context.Context) (*corev1.Service, error) {
	svc, err := c.getWebhookService(ctx)
	if err == nil {
		return svc, nil
	} else if kerrors.IsNotFound(err) {
		return c.createWebhookService(ctx)
	}
	return nil, fmt.Errorf("failed to get-or-create webhook service: %w", err)
}

func (c *Reconciler) getWebhookService(ctx context.Context) (*corev1.Service, error) {
	service := corev1.Service{}
	if err := c.reader.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: c.ns}, &service); err != nil {
		return nil, fmt.Errorf("failed to get webhook service: %w", err)
	}
	return &service, nil
}

func (c *Reconciler) createWebhookService(ctx context.Context) (*corev1.Service, error) {
	err := c.cli.Create(ctx, defaultWebhookService(c.ns))
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create webhook service: %w", err)
	}
	return c.getWebhookService(ctx)
}

func (c *Reconciler) updateWebhook(ctx context.Context, bundle *Bundle) error {
	webhook := v1.MutatingWebhookConfiguration{}
	if err := c.reader.Get(ctx, client.ObjectKey{Name: webhookConfigurationName}, &webhook); err != nil {
		return fmt.Errorf("failed to get webhook: %w", err)
	}

	for i, wh := range webhook.Webhooks {
		if wh.Name == webhookName {
			webhook.Webhooks[i].ClientConfig.CABundle = encodeCertificateFromBundle(bundle)
		}
	}

	if err := c.cli.Update(ctx, &webhook); err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
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

func defaultCertificateSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certificateSecretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"ca.crt":  {},
			"tls.crt": {},
			"tls.key": {},
		},
	}
}

func defaultWebhookService(namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"control-plane": "controller-manager",
			},
			Ports: []corev1.ServicePort{
				{
					Port:       443,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(9443),
				},
			},
		},
	}
}
