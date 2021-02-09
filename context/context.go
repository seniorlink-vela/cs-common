package context

import (
	"context"

	"go.uber.org/zap"
)

const (
	requestIDKey = "request-id"
	loggerKey    = "logger"
)

func GetContextLogger(ctx context.Context) (logger *zap.Logger) {
	if val := ctx.Value(loggerKey); val != nil {
		logger, _ = val.(*zap.Logger)
	}
	return
}

func GetContextRequestID(ctx context.Context) (requestID string) {
	if val := ctx.Value(requestIDKey); val != nil {
		requestID, _ = val.(string)
	}
	return
}

func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func ContextWithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}
