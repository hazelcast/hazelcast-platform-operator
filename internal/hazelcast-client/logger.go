package client

import (
	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client/logger"
	hazelcastv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type LogrHzClientLoggerAdapter struct {
	logger     logr.Logger
	weight     logger.Weight
	enableFunc func() bool
}

func NewLogrHzClientLoggerAdapter(rootLogger logr.Logger, level logger.Level, h *hazelcastv1beta1.Hazelcast) (*LogrHzClientLoggerAdapter, error) {
	w, err := logger.WeightForLogLevel(level)
	l := rootLogger.WithName("client").WithName(h.Name).
		WithValues("Hazelcast", h.Name, "Cluster", h.Spec.ClusterName)
	return &LogrHzClientLoggerAdapter{
		logger:     l,
		weight:     w,
		enableFunc: nil,
	}, err
}

func (l *LogrHzClientLoggerAdapter) Log(weight logger.Weight, formatter func() string) {
	if weight <= l.weight && l.Enabled() {
		l.Logger(weight).Info(formatter())
	}
}

func (l *LogrHzClientLoggerAdapter) Enabled() bool {
	return l.enableFunc != nil && l.enableFunc()
}

func (l *LogrHzClientLoggerAdapter) Logger(weight logger.Weight) logr.Logger {
	if weight <= logger.WeightFatal {
		return l.logger.V(util.PanicLevel).WithValues("Weight", logger.FatalLevel)
	} else if weight <= logger.WeightError {
		return l.logger.V(util.ErrorLevel).WithValues("Weight", logger.ErrorLevel)
	} else if weight <= logger.WeightWarn {
		return l.logger.V(util.WarnLevel).WithValues("Weight", logger.WarnLevel)
	} else if weight <= logger.WeightInfo {
		return l.logger.V(util.InfoLevel).WithValues("Weight", logger.InfoLevel)
	} else if weight <= logger.WeightDebug {
		return l.logger.V(util.DebugLevel).WithValues("Weight", logger.DebugLevel)
	} else {
		return l.logger.V(util.DebugLevel).WithValues("Weight", logger.TraceLevel)
	}
}
