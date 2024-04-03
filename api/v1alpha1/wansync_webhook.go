package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var wansynclog = logf.Log.WithName("wansync-resource")

func (r *WanSync) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-wansync,mutating=false,failurePolicy=fail,sideEffects=None,groups=hazelcast.com,resources=wansyncs,verbs=create;update,versions=v1alpha1,name=vwansync.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &WanSync{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *WanSync) ValidateCreate() (admission.Warnings, error) {
	wansynclog.Info("validate create", "name", r.Name)
	return admission.Warnings{}, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *WanSync) ValidateUpdate(_ runtime.Object) (admission.Warnings, error) {
	wansynclog.Info("validate update", "name", r.Name)
	return admission.Warnings{}, ValidateWanSyncSpec(r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *WanSync) ValidateDelete() (admission.Warnings, error) {
	wansynclog.Info("validate delete", "name", r.Name)
	return admission.Warnings{}, nil
}
