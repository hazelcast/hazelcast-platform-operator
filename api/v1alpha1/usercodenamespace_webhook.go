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

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-usercodenamespace,mutating=false,failurePolicy=fail,sideEffects=None,groups=hazelcast.com,resources=usercodenamespaces,verbs=create;update,versions=v1alpha1,name=vusercodenamespace.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &UserCodeNamespace{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *UserCodeNamespace) ValidateCreate() (admission.Warnings, error) {
	usercodenamespacelog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *UserCodeNamespace) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	usercodenamespacelog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *UserCodeNamespace) ValidateDelete() (admission.Warnings, error) {
	usercodenamespacelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
