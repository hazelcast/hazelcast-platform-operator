package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var topiclog = logf.Log.WithName("topic-resource")

func (r *Topic) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-topic,mutating=false,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=topics,verbs=create;update,versions=v1alpha1,name=vtopic.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Topic{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Topic) ValidateCreate() error {
	topiclog.Info("validate create", "name", r.Name)
	return ValidateTopicSpec(r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Topic) ValidateUpdate(old runtime.Object) error {
	topiclog.Info("validate update", "name", r.Name)
	return ValidateTopicSpec(r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Topic) ValidateDelete() error {
	topiclog.Info("validate delete", "name", r.Name)
	return nil
}
