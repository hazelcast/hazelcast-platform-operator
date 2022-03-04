package certificate

const (
	// certificateSecret is the name of the secret which contains
	// the key and certificate. It will be mounted to the controller.
	// Make sure it is sync with the name in the configuration YAMLs.
	certificateSecret = "webhook-server-cert"

	// webhookService is the name of the service which points to the controller's
	// webhook handler.
	webhookService = "webhook-service"

	// webhookConfigurationName is the name of the webhook configuration.
	webhookConfiguration = "mutating-webhook-configuration"
)

func certificateSecretName(namePrefix string) string {
	return namePrefix + certificateSecret
}

func webhookServiceName(namePrefix string) string {
	return namePrefix + webhookService
}

func webhookConfigurationName(namePrefix string) string {
	return namePrefix + webhookConfiguration
}
