package v1alpha1

import (
	"reflect"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var hazelcastendpointlog = logf.Log.WithName("hazelcastendpoint-resource")

func (r *HazelcastEndpoint) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-hazelcastendpoint,mutating=false,failurePolicy=fail,sideEffects=None,groups=hazelcast.com,resources=hazelcastendpoints,verbs=create;update,versions=v1alpha1,name=vhazelcastendpoint.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &HazelcastEndpoint{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *HazelcastEndpoint) ValidateCreate() (admission.Warnings, error) {
	hazelcastendpointlog.Info("validate create", "name", r.Name)

	return admission.Warnings{}, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *HazelcastEndpoint) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	hazelcastendpointlog.Info("validate update", "name", r.Name)

	if !reflect.DeepEqual(r.Spec, old.(*HazelcastEndpoint).Spec) {
		return admission.Warnings{}, kerrors.NewInvalid(
			schema.GroupKind{Group: "hazelcast.com", Kind: "HazelcastEndpoint"},
			r.Name,
			field.ErrorList{field.Forbidden(Path("spec"), "field cannot be updated")},
		)
	}
	return admission.Warnings{}, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *HazelcastEndpoint) ValidateDelete() (admission.Warnings, error) {
	hazelcastendpointlog.Info("validate delete", "name", r.Name)

	return admission.Warnings{}, nil
}
