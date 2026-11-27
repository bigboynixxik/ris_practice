// Package logger provides structured logging that propagates through context.
package logger

import (
	"context"
	"io"
	"log/slog"
)

type contextKey struct{}

// NewWithWriter returns the CLI's text logger.
func NewWithWriter(output io.Writer, service string) *slog.Logger {
	handler := slog.NewTextHandler(output, nil)
	return slog.New(handler).With("service", service)
}

// NewForEnv uses readable text locally and structured JSON in other environments.
func NewForEnv(env string, output io.Writer, service string) *slog.Logger {
	if env == "local" {
		return NewWithWriter(output, service)
	}
	return slog.New(slog.NewJSONHandler(output, nil)).With("service", service)
}

// WithContext attaches a logger to ctx.
func WithContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, log)
}

// FromContext returns the attached logger or the process default.
func FromContext(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && log != nil {
		return log
	}
	return slog.Default()
}
