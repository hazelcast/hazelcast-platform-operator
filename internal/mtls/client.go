package mtls

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TLSCAKey = "ca.crt"

var (
	errMissingCA      = errors.New("mtls: missing ca.crt in secret")
	errInvalidCA      = errors.New("mtls: failed to find any PEM data in ca input")
	errMissingTLSCert = errors.New("mtls: missing tls.crt in secret")
	errMissingTLSKey  = errors.New("mtls: missing tls.key in secret")
)

type Client struct {
	http.Client
}

func NewClient(ctx context.Context, kubeClient client.Client, secretName types.NamespacedName) (*http.Client, error) {
	if secretName.Namespace == "" {
		// if not specified we always use default namespace
		// as a fallback for running operator locally
		secretName.Namespace = "default"
	}
	secret := &v1.Secret{}
	err := kubeClient.Get(ctx, secretName, secret)
	if err != nil {
		// exit for errors other than not found
		if !kerrors.IsNotFound(err) {
			return nil, err
		}
		// generate new secret with tls cert
		secret, err = createSecret(ctx, kubeClient, secretName)
		if err != nil {
			return nil, err
		}
	}

	// parse secret data
	ca, ok := secret.Data[TLSCAKey]
	if !ok {
		return nil, errMissingCA
	}

	tlsCert, ok := secret.Data[v1.TLSCertKey]
	if !ok {
		return nil, errMissingTLSCert
	}

	tlsKey, ok := secret.Data[v1.TLSPrivateKeyKey]
	if !ok {
		return nil, errMissingTLSKey
	}

	// add ca to pool
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(ca); !ok {
		return nil, errInvalidCA
	}

	// parse tls cert and key
	cert, err := tls.X509KeyPair(tlsCert, tlsKey)
	if err != nil {
		return nil, fmt.Errorf("mtls: %w", err)
	}
	c := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				ServerName:   "mtls",
				RootCAs:      pool,
				Certificates: []tls.Certificate{cert},
			},
		},
	}
	return c, nil
}

func createSecret(ctx context.Context, kubeClient client.Client, secretName types.NamespacedName) (*v1.Secret, error) {
	ca, err := generateCA()
	if err != nil {
		return nil, fmt.Errorf("mtls: %w", err)
	}
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName.Name,
			Namespace: secretName.Namespace,
		},
		Type: v1.SecretTypeTLS,
		Data: ca,
	}
	return secret, kubeClient.Create(ctx, secret)
}

func generateCA() (map[string][]byte, error) {
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
		DNSNames:              []string{"mtls"},
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
		TLSCAKey:            crtPEM.Bytes(),
		v1.TLSCertKey:       crtPEM.Bytes(),
		v1.TLSPrivateKeyKey: keyPEM.Bytes(),
	}, nil
}
