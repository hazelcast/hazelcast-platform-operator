package v1alpha1

import (
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateJetJobSpec(jj *JetJob) error {
	var allErrs field.ErrorList
	println("~~~~~~~~~~~~~~~~~~~~~~~~~~")
	println(jj.Status.Id)
	println(jj.Status.Phase)
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "JetJob"}, jj.Name, allErrs)
}
