package logging

import (
	"context"
	"log/slog"
	"os"
)

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey int

const (
	requestIDKey contextKey = iota
	tenantIDKey
	workflowIDKey
)

// WithRequestID returns a context with the given request id stored.
func WithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, requestIDKey, reqID)
}

// WithTenantID returns a context with the given tenant id stored.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// WithWorkflowID returns a context with the given workflow id stored.
func WithWorkflowID(ctx context.Context, workflowID string) context.Context {
	return context.WithValue(ctx, workflowIDKey, workflowID)
}

// RequestIDFromContext returns the request id stored in ctx, or "" if none.
func RequestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// TenantIDFromContext returns the tenant id stored in ctx, or "" if none.
func TenantIDFromContext(ctx context.Context) string {
	if v := ctx.Value(tenantIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// WorkflowIDFromContext returns the workflow id stored in ctx, or "" if none.
func WorkflowIDFromContext(ctx context.Context) string {
	if v := ctx.Value(workflowIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// FromContext returns a slog.Logger enriched with request, tenant, and workflow ids.
func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	attrs := []any{}
	if reqID := RequestIDFromContext(ctx); reqID != "" {
		attrs = append(attrs, "req_id", reqID)
	}
	if tenantID := TenantIDFromContext(ctx); tenantID != "" {
		attrs = append(attrs, "tenant_id", tenantID)
	}
	if workflowID := WorkflowIDFromContext(ctx); workflowID != "" {
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
