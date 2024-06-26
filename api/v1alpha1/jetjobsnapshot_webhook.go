package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var jetjobsnapshotlog = logf.Log.WithName("jetjobsnapshot-resource")

func (r *JetJobSnapshot) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-jetjobsnapshot,mutating=false,failurePolicy=fail,sideEffects=None,groups=hazelcast.com,resources=jetjobsnapshots,verbs=create;update,versions=v1alpha1,name=vjetjobsnapshot.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &JetJobSnapshot{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (jjs *JetJobSnapshot) ValidateCreate() (admission.Warnings, error) {
	jetjobsnapshotlog.Info("validate create", "name", jjs.Name)
	return admission.Warnings{}, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (jjs *JetJobSnapshot) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	jetjobsnapshotlog.Info("validate update", "name", jjs.Name)
	oldJjs := old.(*JetJobSnapshot)
	return admission.Warnings{}, ValidateJetJobSnapshotSpecUpdate(jjs, oldJjs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (jjs *JetJobSnapshot) ValidateDelete() (admission.Warnings, error) {
	jetjobsnapshotlog.Info("validate delete", "name", jjs.Name)
	return admission.Warnings{}, nil
}
