package certificate

import (
	"context"
	"fmt"

	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *Reconciler) getCertificateSecret(ctx context.Context) (*corev1.Secret, error) {
	secret := corev1.Secret{}
	err := c.reader.Get(ctx, client.ObjectKey{Name: certificateSecretName(c.namePrefix), Namespace: c.ns}, &secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate secret: %w", err)
	}
	return &secret, nil
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

func (c *Reconciler) getWebhookService(ctx context.Context) (*corev1.Service, error) {
	service := corev1.Service{}
	if err := c.reader.Get(ctx, client.ObjectKey{Name: webhookServiceName(c.namePrefix), Namespace: c.ns}, &service); err != nil {
		return nil, fmt.Errorf("failed to get webhook service: %w", err)
	}
	return &service, nil
}

func (c *Reconciler) updateWebhook(ctx context.Context, bundle *Bundle) error {
	webhook, err := c.getWebhookConfiguration(ctx)
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

func (c *Reconciler) getWebhookConfiguration(ctx context.Context) (*admv1.MutatingWebhookConfiguration, error) {
	webhook := admv1.MutatingWebhookConfiguration{}
	if err := c.reader.Get(ctx, client.ObjectKey{Name: webhookConfigurationName(c.namePrefix)}, &webhook); err != nil {
		return nil, fmt.Errorf("failed to get webhook configuration: %w", err)
	}
	return &webhook, nil
}

// triggerPodUpdate adds a temporary annotation to the controller pod and reverts it.
// The reason that this function is called is to trigger a pod update - without restarting the pod - with a temporary
// annotation such that the new files from the secret mount will be reflected to the
// local files quickly. It will reduce the time between the secret update and local files
// update.
func (c *Reconciler) triggerPodUpdate() {
	c.log.Info("Triggering update with annotation")
	pod := corev1.Pod{}
	if err := c.reader.Get(context.Background(), client.ObjectKey{Name: c.name, Namespace: c.ns}, &pod); err != nil {
		c.log.Error(err, "Failed to get controller pod")
		return
	}
	if len(pod.Annotations) == 0 {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations["trigger"] = "true"
	if err := c.cli.Update(context.Background(), &pod); err != nil {
		c.log.Error(err, "Failed to add annotation for pod")
		return
	}
	if err := c.reader.Get(context.Background(), client.ObjectKey{Name: c.name, Namespace: c.ns}, &pod); err != nil {
		c.log.Error(err, "Failed to get controller pod")
		return
	}
	delete(pod.Annotations, "trigger")
	if err := c.cli.Update(context.Background(), &pod); err != nil {
		c.log.Error(err, "Failed to delete annotation for pod")
	}
}
