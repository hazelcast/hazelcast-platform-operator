package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var cachelog = logf.Log.WithName("cache-resource")

func (r *Cache) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-cache,mutating=false,failurePolicy=fail,sideEffects=None,groups=hazelcast.com,resources=caches,verbs=create;update,versions=v1alpha1,name=vcache.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Cache{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Cache) ValidateCreate() error {
	cachelog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Cache) ValidateUpdate(old runtime.Object) error {
	cachelog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Cache) ValidateDelete() error {
	cachelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
