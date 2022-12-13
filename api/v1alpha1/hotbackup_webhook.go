package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var hotbackuplog = logf.Log.WithName("hotbackup-resource")

func (r *HotBackup) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-hotbackup,mutating=false,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=hotbackups,verbs=create;update,versions=v1alpha1,name=vhotbackup.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &HotBackup{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *HotBackup) ValidateCreate() error {
	hotbackuplog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *HotBackup) ValidateUpdate(old runtime.Object) error {
	hotbackuplog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *HotBackup) ValidateDelete() error {
	hotbackuplog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
