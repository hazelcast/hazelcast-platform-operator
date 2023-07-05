package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hazelcast/hazelcast-platform-operator/internal/kubeclient"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func ValidateJetJobCreateSpec(jj *JetJob) error {
	var allErrs field.ErrorList

	if jj.Spec.State != RunningJobState {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("state"),
			jj.Spec.State,
			fmt.Sprintf("should be set to %s on creation", RunningJobState)))
	}

	if jj.Spec.IsBucketEnabled() {
		if jj.Spec.BucketConfiguration.GetSecretName() == "" {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec").Child("bucketConfig").Child("secretName"),
				"bucket secret must be set"))
		} else {
			secretName := types.NamespacedName{
				Name:      jj.Spec.BucketConfiguration.SecretName,
				Namespace: jj.Namespace,
			}
			var secret corev1.Secret
			err := kubeclient.Get(context.Background(), secretName, &secret)
			if kerrors.IsNotFound(err) {
				// we care only about not found error
				allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("bucketConfig").Child("secretName"),
					"Bucket credentials Secret not found"))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "JetJob"}, jj.Name, allErrs)
}

func ValidateExistingJobName(jj *JetJob, jjList *JetJobList) error {
	for _, job := range jjList.Items {
		if job.Name == jj.Name {
			// don't compare to itself
			continue
		}
		if job.JobName() == jj.JobName() && job.Spec.HazelcastResourceName == jj.Spec.HazelcastResourceName {
			return kerrors.NewConflict(schema.GroupResource{Group: "hazelcast.com", Resource: "JetJob"},
				jj.Name, field.Invalid(field.NewPath("spec").Child("name"), job.JobName(),
					fmt.Sprintf("JetJob %s already uses the same name", job.Name)))
		}
	}
	return nil
}

func ValidateJetConfiguration(h *Hazelcast) error {
	var allErrs field.ErrorList
	if !h.Spec.JetEngineConfiguration.IsEnabled() {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("jet").Child("enabled"),
			"jet engine must be enabled"))
	}
	if !h.Spec.JetEngineConfiguration.ResourceUploadEnabled {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("jet").Child("resourceUploadEnabled"),
			h.Spec.JetEngineConfiguration.ResourceUploadEnabled, "jet engine resource upload must be enabled"))
	}
	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Hazelcast"}, h.Name, allErrs)
}

func ValidateJetJobUpdateSpec(jj *JetJob, oldJj *JetJob) error {
	var allErrs = validateJetJobUpdateSpec(jj)
	if err := validateJetStatusChange(jj.Spec.State, oldJj.Status.Phase); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "JetJob"}, jj.Name, allErrs)
}

func validateJetStatusChange(newState JetJobState, oldState JetJobStatusPhase) *field.Error {
	if oldState == "" {
		return nil
	}
	if oldState.IsFinished() || oldState == JetJobCompleting {
		return field.Forbidden(field.NewPath("spec").Child("state"), "job execution is finished or being finished, state change is not allowed")
	}
	if oldState != JetJobRunning && newState != RunningJobState {
		return field.Invalid(field.NewPath("spec").Child("state"), newState, fmt.Sprintf("can be set only for JetJob with %v status", JetJobRunning))
	}
	return nil
}

func validateJetJobUpdateSpec(jj *JetJob) []*field.Error {
	last, ok := jj.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return nil
	}
	var parsed JetJobSpec
	if err := json.Unmarshal([]byte(last), &parsed); err != nil {
		return []*field.Error{field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last JetJob spec for update errors: %w", err))}
	}
	return ValidateJetJobNonUpdatableFields(jj.Spec, parsed)
}

func ValidateJetJobNonUpdatableFields(jj JetJobSpec, oldJj JetJobSpec) []*field.Error {
	var allErrs field.ErrorList
	if jj.Name != oldJj.Name {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("name"), "field cannot be updated"))
	}
	if jj.HazelcastResourceName != oldJj.HazelcastResourceName {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("hazelcastResourceName"), "field cannot be updated"))
	}
	if jj.JarName != oldJj.JarName {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("jarName"), "field cannot be updated"))
	}
	if jj.MainClass != oldJj.MainClass {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("mainClass"), "field cannot be updated"))
	}
	if jj.IsBucketEnabled() != oldJj.IsBucketEnabled() {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("bucketConfiguration"), "field cannot be added or removed"))
	}
	if jj.IsBucketEnabled() && oldJj.IsBucketEnabled() {
		allErrs = append(allErrs,
			ValidateBucketFields(jj.JetRemoteFileConfiguration.BucketConfiguration, oldJj.JetRemoteFileConfiguration.BucketConfiguration)...)
	}
	if jj.IsRemoteURLsEnabled() != oldJj.IsRemoteURLsEnabled() {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("remoteURL"), "field cannot be updated"))
	}
	return allErrs
}

func ValidateBucketFields(jjbc *BucketConfiguration, old *BucketConfiguration) []*field.Error {
	var allErrs field.ErrorList
	if jjbc.BucketURI != old.BucketURI {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("bucketConfiguration").Child("bucketURI"), "field cannot be updated"))
	}
	if jjbc.GetSecretName() != old.GetSecretName() {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("bucketConfiguration").Child("secret"), "field cannot be updated"))
	}
	return allErrs
}
