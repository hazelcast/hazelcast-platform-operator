package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var maplog = logf.Log.WithName("map-resource")

func (m *Map) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-map,mutating=false,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=maps,verbs=create;update,versions=v1alpha1,name=vmap.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Map{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (m *Map) ValidateCreate() error {
	maplog.Info("validate create", "name", m.Name)
	return ValidateMapSpecCreate(m)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (m *Map) ValidateUpdate(old runtime.Object) error {
	maplog.Info("validate update", "name", m.Name)
	return ValidateMapSpecUpdate(m)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (m *Map) ValidateDelete() error {
	maplog.Info("validate delete", "name", m.Name)
	return nil
}
