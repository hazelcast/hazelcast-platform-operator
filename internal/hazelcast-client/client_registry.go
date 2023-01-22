package client

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client/logger"
	"sync"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type ClientRegistry interface {
	Create(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, l logr.Logger) (Client, error)
	Get(ns types.NamespacedName) (Client, bool)
	Delete(ctx context.Context, ns types.NamespacedName) error
}

type HazelcastClientRegistry struct {
	clients sync.Map
}

func (cr *HazelcastClientRegistry) Create(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, l logr.Logger) (Client, error) {
	ns := types.NamespacedName{Namespace: h.Namespace, Name: h.Name}
	client, ok := cr.Get(ns)
	if ok {
		return client, nil
	}
	hzLogger, err := NewLogrHzClientLoggerAdapter(l, logger.InfoLevel, h)
	if err != nil {
		return nil, err
	}
	c, err := NewClient(ctx, BuildConfig(h, hzLogger))
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

func (cr *HazelcastClientRegistry) Delete(ctx context.Context, ns types.NamespacedName) error {
	if c, ok := cr.clients.LoadAndDelete(ns); ok {
		return c.(Client).Shutdown(ctx) //nolint:errcheck
	}
	return nil
}
