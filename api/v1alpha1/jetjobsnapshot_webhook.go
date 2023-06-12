package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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
func (jjs *JetJobSnapshot) ValidateCreate() error {
	jetjobsnapshotlog.Info("validate create", "name", jjs.Name)
	return ValidateJetJobSnapshotSpecCreate(jjs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (jjs *JetJobSnapshot) ValidateUpdate(old runtime.Object) error {
	jetjobsnapshotlog.Info("validate update", "name", jjs.Name)
	oldJjs := old.(*JetJobSnapshot)
	return ValidateJetJobSnapshotSpecUpdate(jjs, oldJjs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (jjs *JetJobSnapshot) ValidateDelete() error {
	jetjobsnapshotlog.Info("validate delete", "name", jjs.Name)
	return nil
}
