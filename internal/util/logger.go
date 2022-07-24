package util

import (
	"context"
	"github.com/go-logr/logr"
)

const (
	PanicLevel int = iota - 3
	ErrorLevel
	WarnLevel
	InfoLevel
	DebugLevel
)

type logKey string

const CtxLogger = logKey("logger")

func GetLogger(ctx context.Context) logr.Logger {
	return ctx.Value(CtxLogger).(logr.Logger)
}
