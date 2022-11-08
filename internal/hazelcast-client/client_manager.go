package client

import (
	"context"
	"errors"
	"sync"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	errNoClient = errors.New("Client is not created yet")
)

type ClientRegistry struct {
	Clients sync.Map
}

func (cs *ClientRegistry) Create(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) (Client, error) {
	ns := types.NamespacedName{Namespace: h.Namespace, Name: h.Name}
	client, err := cs.Get(ns)
	if err == nil {
		return client, nil
	}
	c, err := NewClient(ctx, BuildConfig(h))
	if err == nil {
		return client, nil
	}
	cs.Clients.Store(ns, c)
	return c, nil
}

func (cs *ClientRegistry) Get(ns types.NamespacedName) (client Client, err error) {
	if v, ok := cs.Clients.Load(ns); ok {
		return v.(Client), nil
	}
	return nil, errNoClient
}

func (cs *ClientRegistry) Delete(ctx context.Context, ns types.NamespacedName) {
	if c, ok := cs.Clients.LoadAndDelete(ns); ok {
		c.(Client).Shutdown(ctx) //nolint:errcheck
	}
}
