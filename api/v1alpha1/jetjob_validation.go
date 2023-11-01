package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/kubeclient"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type jetjobValidator struct {
	fieldValidator
}

func NewJetJobValidator(o client.Object) jetjobValidator {
	return jetjobValidator{NewFieldValidator(o)}
}

func ValidateJetJobCreateSpec(jj *JetJob) error {
	v := NewJetJobValidator(jj)

	if jj.Spec.State != RunningJobState {
		v.Invalid(Path("spec", "state"), jj.Spec.State, fmt.Sprintf("should be set to %s on creation", RunningJobState))
	}

	if jj.Spec.IsBucketEnabled() {
		if jj.Spec.BucketConfiguration.GetSecretName() == "" {
			v.Required(Path("spec", "bucketConfig", "secretName"), "bucket secret must be set")
		} else {
			secretName := types.NamespacedName{
				Name:      jj.Spec.BucketConfiguration.SecretName,
				Namespace: jj.Namespace,
			}
			var secret corev1.Secret
			err := kubeclient.Get(context.Background(), secretName, &secret)
			if kerrors.IsNotFound(err) {
				// we care only about not found error
				v.Required(Path("spec", "bucketConfig", "secretName"), "Bucket credentials Secret not found")
			}
		}
	}

	return v.Err()
}

func ValidateExistingJobName(jj *JetJob, jjList *JetJobList) error {
	for _, job := range jjList.Items {
		if job.Name == jj.Name {
			// don't compare to itself
			continue
		}
		if job.JobName() == jj.JobName() && job.Spec.HazelcastResourceName == jj.Spec.HazelcastResourceName {
			return kerrors.NewConflict(schema.GroupResource{Group: "hazelcast.com", Resource: "JetJob"},
				jj.Name, field.Invalid(Path("spec", "name"), job.JobName(),
					fmt.Sprintf("JetJob %s already uses the same name", job.Name)))
		}
	}
	return nil
}

func ValidateJetConfiguration(h *Hazelcast) error {
	v := NewHazelcastValidator(h)
	if !h.Spec.JetEngineConfiguration.IsEnabled() {
		v.Required(Path("spec", "jet", "enabled"), "jet engine must be enabled")
	}
	if !h.Spec.JetEngineConfiguration.ResourceUploadEnabled {
		v.Invalid(Path("spec", "jet", "resourceUploadEnabled"), h.Spec.JetEngineConfiguration.ResourceUploadEnabled, "jet engine resource upload must be enabled")
	}
	return v.Err()
}

func ValidateJetJobUpdateStateSpec(jj *JetJob, oldJj *JetJob) error {
	v := NewJetJobValidator(jj)
	v.validateJetJobUpdateSpec(jj)
	v.validateJetStateChange(jj.Spec.State, oldJj.Spec.State, oldJj.Status.Phase)

	return v.Err()
}

func ValidateJetJobUpdateSpec(jj *JetJob) error {
	v := NewJetJobValidator(jj)
	v.validateJetJobUpdateSpec(jj)
	return v.Err()
}

func (v *jetjobValidator) validateJetStateChange(newState JetJobState, oldState JetJobState, oldStatus JetJobStatusPhase) {
	if newState == oldState || oldStatus == "" {
		return
	}
	if oldStatus.IsFinished() || oldStatus == JetJobCompleting {
		v.Forbidden(Path("spec", "state"), "job execution is finished or being finished, state change is not allowed")
		return
	}
	if oldStatus != JetJobRunning && newState != RunningJobState {
		v.Invalid(Path("spec", "state"), newState, fmt.Sprintf("can be set only for JetJob with %v status", JetJobRunning))
		return
	}
}

func (v *jetjobValidator) validateJetJobUpdateSpec(jj *JetJob) {
	last, ok := jj.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return
	}
	var parsed JetJobSpec
	if err := json.Unmarshal([]byte(last), &parsed); err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last JetJob spec for update errors: %w", err))
		return
	}

	v.ValidateJetJobNonUpdatableFields(jj.Spec, parsed)
}

func (v *jetjobValidator) ValidateJetJobNonUpdatableFields(jj JetJobSpec, oldJj JetJobSpec) {
	if jj.Name != oldJj.Name {
		v.Forbidden(Path("spec", "name"), "field cannot be updated")
	}
	if jj.HazelcastResourceName != oldJj.HazelcastResourceName {
		v.Forbidden(Path("spec", "hazelcastResourceName"), "field cannot be updated")
	}
	if jj.JarName != oldJj.JarName {
		v.Forbidden(Path("spec", "jarName"), "field cannot be updated")
	}
	if jj.MainClass != oldJj.MainClass {
		v.Forbidden(Path("spec", "mainClass"), "field cannot be updated")
	}
	if jj.InitialSnapshotResourceName != oldJj.InitialSnapshotResourceName {
		v.Forbidden(Path("spec", "initialSnapshotResourceName"), "field cannot be updated")
	}
	if jj.IsBucketEnabled() != oldJj.IsBucketEnabled() {
		v.Forbidden(Path("spec", "bucketConfiguration"), "field cannot be added or removed")
	}
	if jj.IsBucketEnabled() && oldJj.IsBucketEnabled() {
		v.validateBucketFields(jj.JetRemoteFileConfiguration.BucketConfiguration, oldJj.JetRemoteFileConfiguration.BucketConfiguration)
	}
	if jj.IsRemoteURLsEnabled() != oldJj.IsRemoteURLsEnabled() {
		v.Forbidden(Path("spec", "remoteURL"), "field cannot be updated")
	}
}

func (v *jetjobValidator) validateBucketFields(jjbc *BucketConfiguration, old *BucketConfiguration) {
	if jjbc.BucketURI != old.BucketURI {
		v.Forbidden(Path("spec", "bucketConfiguration", "bucketURI"), "field cannot be updated")
	}
	if jjbc.GetSecretName() != old.GetSecretName() {
		v.Forbidden(Path("spec", "bucketConfiguration", "secret"), "field cannot be updated")
	}
}
