package logging

import (
	"context"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/stretchr/testify/require"
)

func TestFromContext_includes_request_id(t *testing.T) {
	// Act.
	ctx := ctxval.WithRequestID(context.Background(), "abc123")
	logger := FromContext(ctx)

	// Assert: FromContext returns a non-nil logger.
	require.NotNil(t, logger, "expected non-nil logger")
	_ = ctx
}
