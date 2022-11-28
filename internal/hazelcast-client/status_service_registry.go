package client

import (
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type StatusServiceRegistry interface {
	Create(ns types.NamespacedName, cl Client, l logr.Logger, channel chan event.GenericEvent) StatusService
	Get(ns types.NamespacedName) (StatusService, bool)
	Delete(ns types.NamespacedName)
}

type HzStatusServiceRegistry struct {
	statusServices sync.Map
}

func (ssr *HzStatusServiceRegistry) Create(ns types.NamespacedName, cl Client, l logr.Logger, channel chan event.GenericEvent) StatusService {
	ss, ok := ssr.Get(ns)
	if ok {
		return ss
	}

	ss = NewStatusService(ns, cl, l, channel)
	ssr.statusServices.Store(ns, ss)
	ss.Start()
	return ss
}

func (ssr *HzStatusServiceRegistry) Get(ns types.NamespacedName) (StatusService, bool) {
	if v, ok := ssr.statusServices.Load(ns); ok {
		return v.(StatusService), ok
	}
	return nil, false
}

func (ssr *HzStatusServiceRegistry) Delete(ns types.NamespacedName) {
	if ss, ok := ssr.statusServices.LoadAndDelete(ns); ok {
		ss.(StatusService).Stop()
	}
}
