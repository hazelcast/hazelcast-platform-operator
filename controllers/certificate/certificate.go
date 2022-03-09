package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	oneDay      = 24 * time.Hour
	threeMonths = 90 * oneDay
)

type Bundle struct {
	PrivateKey  *rsa.PrivateKey
	Certificate *x509.Certificate
}

// createSelfSigned generates new CA bundle.
func createSelfSigned(service *corev1.Service) (*Bundle, error) {
	serial, err := rand.Int(rand.Reader, (&big.Int{}).Lsh(big.NewInt(1), uint(127)))
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate serial: %w", err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	now := time.Now()

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "turbine-webhook",
			Organization: []string{"hazelcast"},
		},
		NotBefore:             now.Add(-oneDay),
		NotAfter:              now.Add(threeMonths),
		SignatureAlgorithm:    x509.SHA512WithRSA,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		DNSNames: dnsNames(service),
	}

	certDer, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &Bundle{
		Certificate: cert,
		PrivateKey:  key,
	}, nil
}

// dnsNames returns the required names in the certificate for the webhook service
func dnsNames(service *corev1.Service) []string {
	name := service.Name
	ns := service.Namespace
	return []string{
		fmt.Sprintf("%s.%s.svc", name, ns),
		fmt.Sprintf("%s.%s.svc.cluster.local", name, ns),
	}
}

func encodeCertificateFromBundle(bundle *Bundle) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: bundle.Certificate.Raw,
	})
}

func encodeKeyFromBundle(bundle *Bundle) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(bundle.PrivateKey),
	})
}
