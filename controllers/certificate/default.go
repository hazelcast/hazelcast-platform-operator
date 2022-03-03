package certificate

import (
	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

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

func defaultWebhookConfiguration(namespace string) *admv1.MutatingWebhookConfiguration {
	failure := admv1.FailurePolicyType("Fail")
	none := admv1.SideEffectClass("None")
	return &admv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookConfigurationName,
		},
		Webhooks: []admv1.MutatingWebhook{
			{
				Name:                    webhookName,
				AdmissionReviewVersions: []string{"v1"},
				FailurePolicy:           &failure,
				SideEffects:             &none,
				ClientConfig: admv1.WebhookClientConfig{
					Service: &admv1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      pointer.StringPtr("/inject-turbine"),
					},
				},
				Rules: []admv1.RuleWithOperations{
					{
						Operations: []admv1.OperationType{admv1.Create, admv1.Update},
						Rule: admv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				},
				ObjectSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "turbine.hazelcast.com/name",
							Operator: metav1.LabelSelectorOpExists,
						},
						{
							Key:      "turbine.hazelcast.com/injected",
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{"true"},
						},
					},
				},
			},
		},
	}
}
