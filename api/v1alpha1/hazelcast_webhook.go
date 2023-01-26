package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"encoding/json"
	"fmt"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

// log is for logging in this package.
var hazelcastlog = logf.Log.WithName("hazelcast-resource")

func (r *Hazelcast) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-hazelcast,mutating=false,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=hazelcasts,verbs=create;update,versions=v1alpha1,name=vhazelcast.kb.io,admissionReviewVersions=v1
// Role related to webhooks

var _ webhook.Validator = &Hazelcast{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Hazelcast) ValidateCreate() error {
	hazelcastlog.Info("validate create", "name", r.Name)
	return ValidateHazelcastSpec(r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Hazelcast) ValidateUpdate(old runtime.Object) error {
	hazelcastlog.Info("validate update", "name", r.Name)

	// use last successfully applied spec
	if last, ok := r.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]; ok {
		var parsed HazelcastSpec
		if err := json.Unmarshal([]byte(last), &parsed); err != nil {
			return fmt.Errorf("error parsing last Hazelcast spec: %w", err)
		}
		return ValidateNotUpdatableHazelcastFields(&r.Spec, &parsed)
	}

	if err := ValidateHazelcastSpec(r); err != nil {
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Hazelcast) ValidateDelete() error {
	hazelcastlog.Info("validate delete", "name", r.Name)
	return nil
}
