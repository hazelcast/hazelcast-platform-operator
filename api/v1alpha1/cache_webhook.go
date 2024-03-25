package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var cachelog = logf.Log.WithName("cache-resource")

func (r *Cache) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-cache,mutating=false,failurePolicy=fail,sideEffects=None,groups=hazelcast.com,resources=caches,verbs=create;update,versions=v1alpha1,name=vcache.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Cache{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *Cache) ValidateCreate() (admission.Warnings, error) {
	cachelog.Info("validate create", "name", c.Name)
	return admission.Warnings{}, c.ValidateSpecCreate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *Cache) ValidateUpdate(runtime.Object) (admission.Warnings, error) {
	cachelog.Info("validate update", "name", c.Name)
	return admission.Warnings{}, c.ValidateSpecUpdate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *Cache) ValidateDelete() (admission.Warnings, error) {
	cachelog.Info("validate delete", "name", c.Name)
	return admission.Warnings{}, nil
}
