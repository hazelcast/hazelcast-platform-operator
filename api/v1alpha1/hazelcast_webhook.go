package v1alpha1

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var hazelcastlog = logf.Log.WithName("hazelcast-resource")

func (r *Hazelcast) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-hazelcast,mutating=false,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=hazelcasts,verbs=create;update,versions=v1alpha1,name=vhazelcast.kb.io,admissionReviewVersions=v1
// Role related to webhooks

var _ webhook.Validator = &Hazelcast{}
var _ webhook.Defaulter = &Hazelcast{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Hazelcast) ValidateCreate() error {
	hazelcastlog.Info("validate create", "name", r.Name)
	return ValidateHazelcastSpec(r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Hazelcast) ValidateUpdate(old runtime.Object) error {
	hazelcastlog.Info("validate update", "name", r.Name)
	return ValidateHazelcastSpec(r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Hazelcast) ValidateDelete() error {
	hazelcastlog.Info("validate delete", "name", r.Name)
	return nil
}

func (r *Hazelcast) Default() {
	HzDefaultOptionalToNil(r)
}

func HzDefaultOptionalToNil(r *Hazelcast) {
	if r.Spec.TLS != nil && r.Spec.TLS.SecretName == "" {
		r.Spec.TLS = nil
	}
	if r.Spec.Scheduling != nil && reflect.DeepEqual(*r.Spec.Scheduling, SchedulingConfiguration{}) {
		r.Spec.Scheduling = nil
	}
	if r.Spec.Resources != nil && reflect.DeepEqual(*r.Spec.Resources, corev1.ResourceRequirements{}) {
		r.Spec.Resources = nil
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
}
