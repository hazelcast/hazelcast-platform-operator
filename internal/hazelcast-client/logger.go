package client

import (
	"fmt"
	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client/logger"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type LogrHzClientLoggerAdapter struct {
	logger logr.Logger
	weight logger.Weight
}

func NewLogrHzClientLoggerAdapter(l logr.Logger, level logger.Level, h *hazelcastv1alpha1.Hazelcast) (*LogrHzClientLoggerAdapter, error) {
	w, err := logger.WeightForLogLevel(level)
	clientLogger := l.WithName(fmt.Sprintf("hz-client-%s-%s", h.Name, h.Spec.ClusterName)).
		WithValues("cluster", h.Spec.ClusterName)
	return &LogrHzClientLoggerAdapter{
		logger: clientLogger,
		weight: w,
	}, err
}

func (l *LogrHzClientLoggerAdapter) Log(weight logger.Weight, formatter func() string) {
	if weight <= l.weight {
		l.Logger(weight).Info(formatter())
	}
}

func (l *LogrHzClientLoggerAdapter) Logger(weight logger.Weight) logr.Logger {
	if weight <= logger.WeightFatal {
		return l.logger.V(util.PanicLevel)
	} else if weight <= logger.WeightError {
		return l.logger.V(util.ErrorLevel)
	} else if weight <= logger.WeightWarn {
		return l.logger.V(util.WarnLevel)
	} else if weight <= logger.WeightInfo {
		return l.logger.V(util.InfoLevel)
	} else {
		return l.logger.V(util.DebugLevel)
	}
}
