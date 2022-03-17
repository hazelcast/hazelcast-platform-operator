package certificate

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type namespacedName struct {
	name      string
	namespace string
}

func NewNamespacedNameFilter(name string, namespace string) *namespacedName {
	return &namespacedName{
		name:      name,
		namespace: namespace,
	}
}

func (n *namespacedName) Create(event event.CreateEvent) bool {
	return objectIs(event.Object, n.name, n.namespace)
}

func (n *namespacedName) Delete(event event.DeleteEvent) bool {
	return objectIs(event.Object, n.name, n.namespace)
}

func (n *namespacedName) Update(event event.UpdateEvent) bool {
	return objectIs(event.ObjectNew, n.name, n.namespace)
}

func (n *namespacedName) Generic(event event.GenericEvent) bool {
	return objectIs(event.Object, n.name, n.namespace)
}

func objectIs(object client.Object, name string, namespace string) bool {
	return object.GetNamespace() == namespace && object.GetName() == name
}
