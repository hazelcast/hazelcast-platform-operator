package v1alpha1

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var hazelcastlog = logf.Log.WithName("hazelcast-resource")

func (r *Hazelcast) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-hazelcast,mutating=false,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=hazelcasts,verbs=create;update,versions=v1alpha1,name=vhazelcast.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/mutate-hazelcast-com-v1alpha1-hazelcast,mutating=true,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=hazelcasts,verbs=create;update,versions=v1alpha1,name=vhazelcast.kb.io,admissionReviewVersions=v1
// Role related to webhooks

var _ webhook.Validator = &Hazelcast{}
var _ webhook.Defaulter = &Hazelcast{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Hazelcast) ValidateCreate() (admission.Warnings, error) {
	hazelcastlog.Info("validate create", "name", r.Name)
	return admission.Warnings{}, ValidateHazelcastSpec(r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Hazelcast) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	hazelcastlog.Info("validate update", "name", r.Name)
	if r.GetDeletionTimestamp() != nil {
		return admission.Warnings{}, nil
	}
	return admission.Warnings{}, ValidateHazelcastSpec(r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Hazelcast) ValidateDelete() (admission.Warnings, error) {
	hazelcastlog.Info("validate delete", "name", r.Name)
	return admission.Warnings{}, nil
}

func (r *Hazelcast) Default() {
	if r.Spec.LicenseKeySecretName == "" && r.Spec.DeprecatedLicenseKeySecret != "" {
		r.Spec.LicenseKeySecretName = r.Spec.DeprecatedLicenseKeySecret
		r.Spec.DeprecatedLicenseKeySecret = ""
	}
	r.defaultOptionalToNil()
}

func (r *Hazelcast) defaultOptionalToNil() {
	// Is default TLS
	if r.Spec.TLS != nil && r.Spec.TLS.SecretName == "" && r.Spec.TLS.MutualAuthentication == MutualAuthenticationNone {
		r.Spec.TLS = nil
	}
	if r.Spec.Scheduling != nil && reflect.DeepEqual(*r.Spec.Scheduling, SchedulingConfiguration{}) {
		r.Spec.Scheduling = nil
	}
	if r.Spec.Resources != nil && reflect.DeepEqual(*r.Spec.Resources, corev1.ResourceRequirements{}) {
		r.Spec.Resources = nil
	}
	if r.Spec.Agent.Resources != nil && reflect.DeepEqual(*r.Spec.Agent.Resources, corev1.ResourceRequirements{}) {
		r.Spec.Agent.Resources = nil
	}
	if r.Spec.UserCodeDeployment != nil && reflect.DeepEqual(*r.Spec.UserCodeDeployment, UserCodeDeploymentConfig{}) {
		r.Spec.UserCodeDeployment = nil
	}
	if r.Spec.AdvancedNetwork != nil && reflect.DeepEqual(*r.Spec.AdvancedNetwork, AdvancedNetwork{}) {
		r.Spec.AdvancedNetwork = nil
	}
	if r.Spec.ManagementCenterConfig != nil && reflect.DeepEqual(*r.Spec.ManagementCenterConfig, ManagementCenterConfig{}) {
		r.Spec.ManagementCenterConfig = nil
	}
	if r.Spec.Persistence.Restore != nil && reflect.DeepEqual(r.Spec.Persistence.Restore, RestoreConfiguration{}) {
		r.Spec.Persistence.Restore = nil
	}
}
