package client

import (
	"context"
	"errors"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type StatusServiceRegistry struct {
	statusServices sync.Map
}

var (
	errNoStatusService = errors.New("Status Service is not created yet")
)

func (ssm *StatusServiceRegistry) Create(cl Client, l logr.Logger, ns types.NamespacedName, channel chan event.GenericEvent) *StatusService {
	ss, err := ssm.Get(ns)
	if err == nil {
		return ss
	}

	ss = newMemberStatusService(cl, l, ns, channel)
	ssm.statusServices.Store(ns, ss)
	ss.Start()
	return ss
}

func (ssm *StatusServiceRegistry) Get(ns types.NamespacedName) (client *StatusService, err error) {
	if v, ok := ssm.statusServices.Load(ns); ok {
		return v.(*StatusService), nil
	}
	return nil, errNoStatusService
}

func (ssm *StatusServiceRegistry) Delete(ctx context.Context, ns types.NamespacedName) {
	if ss, ok := ssm.statusServices.LoadAndDelete(ns); ok {
		ss.(*StatusService).Stop(ctx)
	}
}
