package mtls

import (
	"context"
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type HttpClientRegistry interface {
	Create(ctx context.Context, kubeClient client.Client, ns string) (*http.Client, error)
	GetOrCreate(ctx context.Context, kubeClient client.Client, ns string) (*http.Client, error)
	Delete(ns string)
}

func NewHttpClientRegistry() HttpClientRegistry {
	return &httpClientRegistry{}
}

type httpClientRegistry struct {
	clients sync.Map
}

func (cr *httpClientRegistry) Create(ctx context.Context, kubeClient client.Client, ns string) (*http.Client, error) {
	c, ok := cr.get(ns)
	if ok {
		return c, nil
	}
	nn := types.NamespacedName{Name: n.MTLSCertSecretName, Namespace: ns}
	c, err := NewClient(ctx, kubeClient, nn)
	if err != nil {
		return nil, err
	}
	cr.clients.Store(nn, c)
	return c, nil
}

func (cr *httpClientRegistry) GetOrCreate(ctx context.Context, kubeClient client.Client, ns string) (*http.Client, error) {
	nn := types.NamespacedName{Name: n.MTLSCertSecretName, Namespace: ns}
	if v, ok := cr.clients.Load(nn); ok {
		return v.(*http.Client), nil
	}
	return cr.Create(ctx, kubeClient, ns)
}

func (cr *httpClientRegistry) Delete(ns string) {
	cr.clients.Delete(types.NamespacedName{Name: n.MTLSCertSecretName, Namespace: ns})
}

func (cr *httpClientRegistry) get(ns string) (*http.Client, bool) {
	nn := types.NamespacedName{Name: n.MTLSCertSecretName, Namespace: ns}
	if v, ok := cr.clients.Load(nn); ok {
		return v.(*http.Client), true
	}
	return nil, false
}
