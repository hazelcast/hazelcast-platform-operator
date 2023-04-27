package client

import (
	"context"
	"crypto/x509"
	"sync"

	"github.com/hazelcast/hazelcast-go-client/logger"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type ClientRegistry interface {
	GetOrCreate(ctx context.Context, nn types.NamespacedName) (Client, error)
	Get(ns types.NamespacedName) (Client, bool)
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
	client, ok := cr.Get(nn)
	if ok {
		return client, nil
	}
	hzLogger, err := NewLogrHzClientLoggerAdapter(ctrl.Log, logger.ErrorLevel, h)
	if err != nil {
		return nil, err
	}
	var pool *x509.CertPool
	if h.Spec.TLS.IsEnabled() {
		var s v1.Secret
		err = cr.K8sClient.Get(ctx, types.NamespacedName{Name: h.Spec.TLS.SecretName, Namespace: h.Namespace}, &s)
		if err != nil {
			return nil, err
		}
		pool = x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(s.Data[v1.TLSCertKey]); !ok {
			return nil, err
		}
	}
	c, err := NewClient(ctx, BuildConfig(h, pool, hzLogger))
	if err != nil {
		return nil, err
	}
	cr.clients.Store(nn, c)
	hzLogger.enableFunc = c.IsClientConnected
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
