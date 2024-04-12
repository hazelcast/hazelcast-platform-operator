package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var usercodenamespacelog = logf.Log.WithName("usercodenamespace-resource")

func (r *UserCodeNamespace) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-usercodenamespace,mutating=false,failurePolicy=fail,sideEffects=None,groups=hazelcast.com,resources=usercodenamespaces,verbs=create;update,versions=v1alpha1,name=vusercodenamespace.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &UserCodeNamespace{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *UserCodeNamespace) ValidateCreate() (admission.Warnings, error) {
	usercodenamespacelog.Info("validate create", "name", r.Name)
	return admission.Warnings{}, ValidateUCNSpecCreate(r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *UserCodeNamespace) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	usercodenamespacelog.Info("validate update", "name", r.Name)
	return admission.Warnings{}, ValidateUCNSpecUpdate(r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *UserCodeNamespace) ValidateDelete() (admission.Warnings, error) {
	usercodenamespacelog.Info("validate delete", "name", r.Name)
	return admission.Warnings{}, nil
}
