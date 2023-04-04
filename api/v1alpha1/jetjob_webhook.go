package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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
func (jj *JetJob) ValidateCreate() error {
	jetjoblog.Info("validate create", "name", jj.Name)
	// 1. Hazelcast Jet resource upload must be enabled
	// 2. The Job name within a HZ cluster must be unique
	return ValidateJetJobSpec(jj)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (jj *JetJob) ValidateUpdate(old runtime.Object) error {
	jetjoblog.Info("validate update", "name", jj.Name)
	oldJj := old.(*JetJob)
	println(fmt.Sprintf("%v", oldJj))
	return ValidateJetJobSpec(jj)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (jj *JetJob) ValidateDelete() error {
	jetjoblog.Info("validate delete", "name", jj.Name)
	return nil
}
