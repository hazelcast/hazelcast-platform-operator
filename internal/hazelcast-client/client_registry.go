package client

import (
	"context"
	"sync"

	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type ClientRegistry interface {
	GetOrCreate(ctx context.Context, nn types.NamespacedName) (Client, error)
	Delete(ctx context.Context, ns types.NamespacedName) error
}

type HazelcastClientRegistry struct {
	clients   sync.Map
	K8sClient k8sClient.Client
}

func (cr *HazelcastClientRegistry) GetOrCreate(ctx context.Context, nn types.NamespacedName) (Client, error) {
	h := &hazelcastv1alpha1.Hazelcast{}
	err := cr.K8sClient.Get(ctx, nn, h)
	if err != nil {
		return nil, err
	}

	client, ok := cr.get(nn)
	if ok {
		return client, nil
	}
	c, err := NewClient(ctx, BuildConfig(h))
	if err != nil {
		return nil, err
	}
	cr.clients.Store(nn, c)
	return c, nil
}

func (cr *HazelcastClientRegistry) get(ns types.NamespacedName) (Client, bool) {
	if v, ok := cr.clients.Load(ns); ok {
		return v.(Client), true
	}
	return nil, false
}

func (cr *HazelcastClientRegistry) Delete(ctx context.Context, ns types.NamespacedName) error {
	if c, ok := cr.clients.LoadAndDelete(ns); ok {
		return c.(Client).Shutdown(ctx) //nolint:errcheck
	}
	return nil
}
