package mtls

import (
	"context"
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HttpClientRegistry interface {
	Get(ctx context.Context, name types.NamespacedName) (*http.Client, bool)
	Delete(name types.NamespacedName)
}

func NewHttpClientRegistry(kubeClient client.Client) HttpClientRegistry {
	return &httpClientRegistry{
		kubeClient: kubeClient,
	}
}

type httpClientRegistry struct {
	kubeClient client.Client
	clients    sync.Map
}

func (cr *httpClientRegistry) Get(ctx context.Context, name types.NamespacedName) (*http.Client, bool) {
	v, ok := cr.clients.Load(name)
	if !ok {
		c, err := NewClient(ctx, cr.kubeClient, name)
		if err != nil {
			return nil, false
		}
		cr.clients.Store(name, c)
		return c, true
	}
	return v.(*http.Client), true
}

func (cr *httpClientRegistry) Delete(name types.NamespacedName) {
	cr.clients.Delete(name)
}
