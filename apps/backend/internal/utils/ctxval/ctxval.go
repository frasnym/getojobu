package ctxval

import (
	"context"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/jackc/pgx/v5"
)

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey int

const (
	requestIDKey contextKey = iota
	tenantIDKey
	workflowIDKey
	principalKey
	primaryTransactionKey
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

// ContextWithPrincipal returns a context with the Principal attached.
func ContextWithPrincipal(ctx context.Context, p *models.Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// PrincipalFromContext returns the Principal stored in ctx, or nil if none.
func PrincipalFromContext(ctx context.Context) *models.Principal {
	p, _ := ctx.Value(principalKey).(*models.Principal)
	return p
}

// SetPrimaryTransaction stores a primary transaction in ctx.
func SetPrimaryTransaction(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, primaryTransactionKey, tx)
}

// GetPrimaryTransaction retrieves the primary transaction from ctx, or returns
// constants.ErrNoActiveConnections when no transaction is stored.
func GetPrimaryTransaction(ctx context.Context) (pgx.Tx, error) {
	if ctx == nil {
		return nil, constants.ErrNoActiveConnections
	}
	tx, ok := ctx.Value(primaryTransactionKey).(pgx.Tx)
	if !ok || tx == nil {
		return nil, constants.ErrNoActiveConnections
	}
	return tx, nil
}
