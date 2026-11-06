package logging

import (
	"context"
	"log/slog"
	"os"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
)

// FromContext returns a slog.Logger enriched with request, tenant, and workflow ids.
func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	attrs := []any{}
	if reqID := ctxval.RequestIDFromContext(ctx); reqID != "" {
		attrs = append(attrs, "req_id", reqID)
	}
	if tenantID := ctxval.TenantIDFromContext(ctx); tenantID != "" {
		attrs = append(attrs, "tenant_id", tenantID)
	}
	if workflowID := ctxval.WorkflowIDFromContext(ctx); workflowID != "" {
		attrs = append(attrs, "workflow_id", workflowID)
	}
	if len(attrs) > 0 {
		logger = logger.With(attrs...)
	}
	return logger
}

// New creates a base structured logger writing JSON to stdout.
func New(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

// Error logs msg enriched with request/tenant/workflow context from ctx,
// attaching the error under the "error" key. It is the single point handlers
// should call to log failures, so swapping the underlying logging library
// only requires changing this package.
func Error(ctx context.Context, msg string, err error) {
	FromContext(ctx).Error(msg, "error", err)
}

// Info logs msg enriched with request/tenant/workflow context from ctx,
// with any additional key-value pairs passed as args.
func Info(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Info(msg, args...)
}

// Warn logs msg enriched with request/tenant/workflow context from ctx,
// with any additional key-value pairs passed as args.
func Warn(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Warn(msg, args...)
}

// Debug logs msg enriched with request/tenant/workflow context from ctx,
// with any additional key-value pairs passed as args.
func Debug(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Debug(msg, args...)
}
