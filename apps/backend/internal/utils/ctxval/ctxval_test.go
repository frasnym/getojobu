package ctxval

import (
	"context"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/jackc/pgx/v5"
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

func TestContextWithPrincipal_and_PrincipalFromContext(t *testing.T) {
	// Act.
	ctx := ContextWithPrincipal(context.Background(), &models.Principal{
		ID:   "user-1",
		Role: models.RoleUser,
	})

	// Assert.
	p := PrincipalFromContext(ctx)
	require.NotNil(t, p)
	assert.Equal(t, "user-1", p.ID)
	assert.Equal(t, models.RoleUser, p.Role)

	// Assert: nil when no principal is set.
	assert.Nil(t, PrincipalFromContext(context.Background()))
}

type dummyTx struct{ pgx.Tx }

func TestPrimaryTransaction_stores_and_retrieves(t *testing.T) {
	// Assert: no active transaction on fresh context.
	_, err := GetPrimaryTransaction(context.Background())
	assert.ErrorIs(t, err, constants.ErrNoActiveConnections)

	// Act.
	tx := dummyTx{}
	ctx := SetPrimaryTransaction(context.Background(), tx)

	// Assert.
	got, err := GetPrimaryTransaction(ctx)
	require.NoError(t, err)
	assert.Equal(t, tx, got)
}
