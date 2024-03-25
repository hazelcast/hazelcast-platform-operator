package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var queuelog = logf.Log.WithName("queue-resource")

func (r *Queue) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-queue,mutating=false,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=queues,verbs=create;update,versions=v1alpha1,name=vqueue.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Queue{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Queue) ValidateCreate() (admission.Warnings, error) {
	queuelog.Info("validate create", "name", r.Name)
	return admission.Warnings{}, r.ValidateSpecCreate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Queue) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	queuelog.Info("validate update", "name", r.Name)
	return admission.Warnings{}, r.ValidateSpecUpdate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Queue) ValidateDelete() (admission.Warnings, error) {
	queuelog.Info("validate delete", "name", r.Name)
	return admission.Warnings{}, nil
}
