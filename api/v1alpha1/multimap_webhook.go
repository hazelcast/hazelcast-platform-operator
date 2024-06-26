package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var multimaplog = logf.Log.WithName("multimap-resource")

func (r *MultiMap) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-multimap,mutating=false,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=multimaps,verbs=create;update,versions=v1alpha1,name=vmultimap.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &MultiMap{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MultiMap) ValidateCreate() (admission.Warnings, error) {
	multimaplog.Info("validate create", "name", r.Name)
	return admission.Warnings{}, r.ValidateSpecCreate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MultiMap) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	multimaplog.Info("validate update", "name", r.Name)
	return admission.Warnings{}, r.ValidateSpecUpdate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MultiMap) ValidateDelete() (admission.Warnings, error) {
	multimaplog.Info("validate delete", "name", r.Name)
	return admission.Warnings{}, nil
}
