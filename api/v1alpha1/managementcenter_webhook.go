package v1alpha1

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var managementcenterlog = logf.Log.WithName("managementcenter-resource")

func (r *ManagementCenter) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-hazelcast-com-v1alpha1-managementcenter,mutating=false,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=managementcenters,verbs=create;update,versions=v1alpha1,name=vmanagementcenter.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/mutate-hazelcast-com-v1alpha1-managementcenter,mutating=true,failurePolicy=ignore,sideEffects=None,groups=hazelcast.com,resources=managementcenters,verbs=create;update,versions=v1alpha1,name=mmanagementcenter.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ManagementCenter{}
var _ webhook.Defaulter = &ManagementCenter{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ManagementCenter) ValidateCreate() (admission.Warnings, error) {
	managementcenterlog.Info("validate create", "name", r.Name)
	return admission.Warnings{}, ValidateManagementCenterSpec(r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ManagementCenter) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	managementcenterlog.Info("validate update", "name", r.Name)
	if r.GetDeletionTimestamp() != nil {
		return admission.Warnings{}, nil
	}
	return admission.Warnings{}, ValidateManagementCenterSpec(r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ManagementCenter) ValidateDelete() (admission.Warnings, error) {
	managementcenterlog.Info("validate delete", "name", r.Name)
	return admission.Warnings{}, nil
}

func (r *ManagementCenter) Default() {
	r.defaultOptionalToNil()
}

func (r *ManagementCenter) defaultOptionalToNil() {
	if r.Spec.ExternalConnectivity != nil && reflect.DeepEqual(*r.Spec.ExternalConnectivity, ExternalConnectivityConfiguration{}) {
		r.Spec.ExternalConnectivity = nil
	}
	if r.Spec.Persistence != nil && reflect.DeepEqual(*r.Spec.Persistence, MCPersistenceConfiguration{}) {
		r.Spec.Persistence = nil
	}
	if r.Spec.Scheduling != nil && reflect.DeepEqual(*r.Spec.Scheduling, SchedulingConfiguration{}) {
		r.Spec.Scheduling = nil
	}
	if r.Spec.Resources != nil && reflect.DeepEqual(*r.Spec.Resources, corev1.ResourceRequirements{}) {
		r.Spec.Resources = nil
	}
	if r.Spec.HazelcastClusters != nil {
		for i, cluster := range r.Spec.HazelcastClusters {
			// Is default TLS
			if cluster.TLS != nil && cluster.TLS.SecretName == "" && cluster.TLS.MutualAuthentication == MutualAuthenticationNone {
				r.Spec.HazelcastClusters[i].TLS = nil
			}
		}
	}
}
