package webhookca

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	admissionregistration "k8s.io/api/admissionregistration/v1"
	core "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	certManagerAnnotation = "cert-manager.io/inject-ca-from"
	webhookServerPath     = "/tmp/k8s-webhook-server/serving-certs"
)

type CAInjector struct {
	kubeClient   client.Client
	webhookNames []types.NamespacedName
	tlsCert      []byte
}

func NewCAInjector(kubeClient client.Client, deploymentName, namespace string) (*CAInjector, error) {
	c := CAInjector{
		kubeClient:   kubeClient,
		webhookNames: webhookNames(deploymentName, namespace),
	}

	certPath := filepath.Join(webhookServerPath, core.TLSCertKey)
	if _, err := os.Stat(certPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}

		sn := serviceName(deploymentName, namespace)

		// tls.crt not found, we need to create new
		ca, err := generateCA(sn.Name, sn.Namespace)
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
	// olm is using multiple webhook configurations and we need
	// working kubernetes client to list them
	olmWebhooks, err := olmWebhookNames(ctx, c.kubeClient)
	if err != nil {
		return err
	}
	c.webhookNames = append(c.webhookNames, olmWebhooks...)

	// try to update webhook certificates
	for _, w := range c.webhookNames {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			config := admissionregistration.ValidatingWebhookConfiguration{}
			if err := c.kubeClient.Get(ctx, w, &config); err != nil {
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
		if err != nil {
			if kerrors.IsNotFound(err) {
				// skip if the webhook is missing (possible in olm)
				continue
			}
			return err
		}
	}
	return nil
}

func generateCA(name, namespace string) (map[string][]byte, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization: []string{"Hazelcast"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		DNSNames: []string{
			// default
			fmt.Sprintf("%s.%s.svc", name, namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", name, namespace),
			// for olm
			fmt.Sprintf("hazelcast-platform-controller-manager-service.%s.svc", namespace),
			fmt.Sprintf("hazelcast-platform-controller-manager-service.%s.svc.cluster.local", namespace),
		},
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	rawCrt, err := x509.CreateCertificate(rand.Reader, ca, ca, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	var crtPEM bytes.Buffer
	err = pem.Encode(&crtPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rawCrt,
	})
	if err != nil {
		return nil, err
	}

	var keyPEM bytes.Buffer
	err = pem.Encode(&keyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		core.TLSCertKey:       crtPEM.Bytes(),
		core.TLSPrivateKeyKey: keyPEM.Bytes(),
	}, nil
}

func webhookNames(deploymentName, namespace string) []types.NamespacedName {
	webhookName := types.NamespacedName{
		Name:      deploymentName,
		Namespace: namespace,
	}

	if webhookName.Namespace == "" {
		webhookName.Namespace = "default"
	}

	return []types.NamespacedName{webhookName}
}

func olmWebhookNames(ctx context.Context, kubeClient client.Client) ([]types.NamespacedName, error) {
	labels := client.MatchingLabels{
		"olm.owner.namespace": "hazelcast-platform-operator",
	}

	configs := &admissionregistration.ValidatingWebhookConfigurationList{}
	if err := kubeClient.List(ctx, configs, labels); err != nil {
		return nil, err
	}

	var webhookNames []types.NamespacedName
	for _, config := range configs.Items {
		webhookNames = append(webhookNames, types.NamespacedName{
			Name:      config.Name,
			Namespace: config.Namespace,
		})
	}

	return webhookNames, nil
}

func serviceName(deploymentName, namespace string) types.NamespacedName {
	serviceName := types.NamespacedName{
		Name:      deploymentName,
		Namespace: namespace, // service namespace is also hardcoded in webhook manifest
	}

	if serviceName.Namespace == "" {
		serviceName.Namespace = "default"
	}

	return serviceName
}
