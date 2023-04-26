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

func NewClient(ctx context.Context, kubeClient client.Client, secretName types.NamespacedName) (*http.Client, error) {
	secret := &v1.Secret{}
	err := kubeClient.Get(ctx, secretName, secret)
	if err != nil {
		return nil, err
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

func NewCertificateAuthority() (cert []byte, key []byte, err error) {
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
		return nil, nil, err
	}

	rawCrt, err := x509.CreateCertificate(rand.Reader, ca, ca, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	var crtPEM bytes.Buffer
	err = pem.Encode(&crtPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rawCrt,
	})
	if err != nil {
		return nil, nil, err
	}

	var keyPEM bytes.Buffer
	err = pem.Encode(&keyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err != nil {
		return nil, nil, err
	}

	return crtPEM.Bytes(), keyPEM.Bytes(), nil
}
