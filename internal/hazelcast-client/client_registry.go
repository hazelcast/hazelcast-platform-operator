package client

import (
	"context"
	"sync"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type ClientRegistry interface {
	Create(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) (Client, error)
	Get(ns types.NamespacedName) (Client, bool)
	Delete(ctx context.Context, ns types.NamespacedName)
}

type HazelcastClientRegistry struct {
	clients sync.Map
}

func (cr *HazelcastClientRegistry) Create(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) (Client, error) {
	ns := types.NamespacedName{Namespace: h.Namespace, Name: h.Name}
	client, ok := cr.Get(ns)
	if ok {
		return client, nil
	}
	c, err := NewClient(ctx, BuildConfig(h))
	if err != nil {
		return nil, err
	}
	cr.clients.Store(ns, c)
	return c, nil
}

func (cr *HazelcastClientRegistry) Get(ns types.NamespacedName) (Client, bool) {
	if v, ok := cr.clients.Load(ns); ok {
		return v.(Client), true
	}
	return nil, false
}

func (cr *HazelcastClientRegistry) Delete(ctx context.Context, ns types.NamespacedName) {
	if c, ok := cr.clients.LoadAndDelete(ns); ok {
		c.(Client).Shutdown(ctx) //nolint:errcheck
	}
}
