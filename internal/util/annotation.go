package util

import (
	"encoding/json"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func InsertLastSuccessfullyAppliedSpec(spec interface{}, wan client.Object) client.Object {
	return insertSpec(spec, n.LastSuccessfulSpecAnnotation, wan)
}

func InsertLastAppliedSpec(spec interface{}, wan client.Object) client.Object {
	return insertSpec(spec, n.LastAppliedSpecAnnotation, wan)
}

func IsApplied(wan v1.ObjectMeta) bool {
	_, ok := wan.Annotations[n.LastAppliedSpecAnnotation]
	return ok
}

func IsSuccessfullyApplied(wan v1.ObjectMeta) bool {
	_, ok := wan.Annotations[n.LastSuccessfulSpecAnnotation]
	return ok
}

func insertSpec(spec interface{}, annotation string, wan client.Object) client.Object {
	b, _ := json.Marshal(spec)
	annotations := wan.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[annotation] = string(b)
	wan.SetAnnotations(annotations)
	return wan
}
