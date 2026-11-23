// Package logger stores the application's structured logger in a context.
package logger

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// WithContext attaches a logger to ctx.
func WithContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, log)
}

// FromContext returns the attached logger or the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return log
	}
	return slog.Default()
}
