package certificate

import (
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
)

func certificateSecretName(namePrefix string) string {
	return namePrefix + n.CertificateSecret
}

func webhookServiceName(namePrefix string) string {
	return namePrefix + n.WebhookService
}

func webhookConfigurationName(namePrefix string) string {
	return namePrefix + n.WebhookConfiguration
}
