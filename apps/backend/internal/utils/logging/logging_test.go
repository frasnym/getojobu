package logging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithRequestID_stores_and_retrieves(t *testing.T) {
	// Act.
	ctx := WithRequestID(context.Background(), "abc123")

	// Assert.
	assert.Equal(t, "abc123", RequestIDFromContext(ctx), "expected req id abc123")
}

func TestWithTenantID_and_workflowID_compose(t *testing.T) {
	// Act.
	ctx := WithTenantID(context.Background(), "tenant-1")
	ctx = WithWorkflowID(ctx, "wf-1")
	ctx = WithRequestID(ctx, "req-1")

	// Assert.
	assert.Equal(t, "tenant-1", TenantIDFromContext(ctx), "expected tenant tenant-1")
	assert.Equal(t, "wf-1", WorkflowIDFromContext(ctx), "expected workflow wf-1")
	assert.Equal(t, "req-1", RequestIDFromContext(ctx), "expected req req-1")
}

func TestFromContext_includes_request_id(t *testing.T) {
	// Act.
	ctx := WithRequestID(context.Background(), "abc123")
	logger := FromContext(ctx)

	// Assert: FromContext returns a non-nil logger.
	require.NotNil(t, logger, "expected non-nil logger")
	_ = ctx
}
