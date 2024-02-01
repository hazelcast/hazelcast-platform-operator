package controllers

import (
	"encoding/json"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func InsertLastSuccessfullyAppliedSpec(spec interface{}, obj client.Object) client.Object {
	return insertSpec(spec, n.LastSuccessfulSpecAnnotation, obj)
}

func InsertLastAppliedSpec(spec interface{}, obj client.Object) client.Object {
	return insertSpec(spec, n.LastAppliedSpecAnnotation, obj)
}

func IsApplied(meta v1.ObjectMeta) bool {
	_, ok := meta.Annotations[n.LastAppliedSpecAnnotation]
	return ok
}

func IsSuccessfullyApplied(obj client.Object) bool {
	_, ok := obj.GetAnnotations()[n.LastSuccessfulSpecAnnotation]
	return ok
}

func insertSpec(spec interface{}, annotation string, obj client.Object) client.Object {
	b, _ := json.Marshal(spec)
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[annotation] = string(b)
	obj.SetAnnotations(annotations)
	return obj
}
