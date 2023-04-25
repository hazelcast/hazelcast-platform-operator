package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var managementcenterlog = logf.Log.WithName("managementcenter-resource")

func (r *ManagementCenter) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-managementcenter,mutating=false,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=managementcenters,verbs=create;update,versions=v1alpha1,name=vmanagementcenter.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ManagementCenter{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ManagementCenter) ValidateCreate() error {
	managementcenterlog.Info("validate create", "name", r.Name)
	if err := ValidateManagementCenterSpec(r); err != nil {
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ManagementCenter) ValidateUpdate(old runtime.Object) error {
	managementcenterlog.Info("validate update", "name", r.Name)
	if err := ValidateManagementCenterSpec(r); err != nil {
		return err
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ManagementCenter) ValidateDelete() error {
	managementcenterlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
