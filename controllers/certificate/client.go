package certificate

import (
	"context"
	"fmt"

	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
	webhook, err := c.getOrCreateWebhookConfiguration(ctx)
	if err != nil {
		return err
	}

	for i, wh := range webhook.Webhooks {
		if wh.Name == webhookName {
			webhook.Webhooks[i].ClientConfig.CABundle = encodeCertificateFromBundle(bundle)
		}
	}

	if err := c.cli.Update(ctx, webhook); err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
}

func (c *Reconciler) getOrCreateWebhookConfiguration(ctx context.Context) (*admv1.MutatingWebhookConfiguration, error) {
	svc, err := c.getWebhookConfiguration(ctx)
	if err == nil {
		return svc, nil
	} else if kerrors.IsNotFound(err) {
		return c.createWebhookConfiguration(ctx)
	}
	return nil, fmt.Errorf("failed to get-or-create webhook configuration: %w", err)
}

func (c *Reconciler) getWebhookConfiguration(ctx context.Context) (*admv1.MutatingWebhookConfiguration, error) {
	webhook := admv1.MutatingWebhookConfiguration{}
	if err := c.reader.Get(ctx, client.ObjectKey{Name: webhookConfigurationName}, &webhook); err != nil {
		return nil, fmt.Errorf("failed to get webhook configuration: %w", err)
	}
	return &webhook, nil
}

func (c *Reconciler) createWebhookConfiguration(ctx context.Context) (*admv1.MutatingWebhookConfiguration, error) {
	err := c.cli.Create(ctx, defaultWebhookConfiguration(c.ns))
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create webhook configuraiton: %w", err)
	}
	return c.getWebhookConfiguration(ctx)
}
