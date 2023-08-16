package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var jetjoblog = logf.Log.WithName("jetjob-resource")

func (r *JetJob) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-jetjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=hazelcast.com,resources=jetjobs,verbs=create;update,versions=v1alpha1,name=vjetjob.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &JetJob{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (jj *JetJob) ValidateCreate() (admission.Warnings, error) {
	jetjoblog.Info("validate create", "name", jj.Name)
	return admission.Warnings{}, ValidateJetJobCreateSpec(jj)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (jj *JetJob) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	jetjoblog.Info("validate update", "name", jj.Name)
	oldJj := old.(*JetJob)
	return admission.Warnings{}, ValidateJetJobUpdateSpec(jj, oldJj)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (jj *JetJob) ValidateDelete() (admission.Warnings, error) {
	jetjoblog.Info("validate delete", "name", jj.Name)
	return admission.Warnings{}, nil
}
