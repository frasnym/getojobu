package database

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
)

type mockTx struct {
	committed  bool
	rolledBack bool
	commitErr  error
}

func (m *mockTx) Begin(ctx context.Context) (pgx.Tx, error) { return m, nil }
func (m *mockTx) Commit(ctx context.Context) error {
	m.committed = true
	return m.commitErr
}
func (m *mockTx) Rollback(ctx context.Context) error {
	m.rolledBack = true
	return nil
}
func (m *mockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (m *mockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (m *mockTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (m *mockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (m *mockTx) Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error) {
	return pgconn.CommandTag{}, nil
}
func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row { return nil }
func (m *mockTx) Conn() *pgx.Conn                                               { return nil }

type mockDB struct {
	tx       *mockTx
	beginErr error
}

func (d *mockDB) Ping(ctx context.Context) error { return nil }
func (d *mockDB) Close()                         {}
func (d *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}
func (d *mockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (d *mockDB) Begin(ctx context.Context) (pgx.Tx, error) {
	if d.beginErr != nil {
		return nil, d.beginErr
	}
	return d.tx, nil
}

func TestTransact_success_commits(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}

	err := Transact(context.Background(), db, func(ctx context.Context) error {
		activeTx, err := ctxval.GetPrimaryTransaction(ctx)
		require.NoError(t, err)
		assert.Equal(t, tx, activeTx)
		return nil
	})

	require.NoError(t, err)
	assert.True(t, tx.committed)
	assert.False(t, tx.rolledBack)
}

func TestTransact_error_rolls_back(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}
	expectedErr := errors.New("operation failed")

	err := Transact(context.Background(), db, func(ctx context.Context) error {
		return expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
	assert.False(t, tx.committed)
	assert.True(t, tx.rolledBack)
}

func TestTransact_panic_rolls_back_and_repanics(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}

	assert.PanicsWithValue(t, "boom", func() {
		_ = Transact(context.Background(), db, func(ctx context.Context) error {
			panic("boom")
		})
	})

	assert.False(t, tx.committed)
	assert.True(t, tx.rolledBack)
}

func TestTransact_nested_child_reuses_parent(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}

	err := Transact(context.Background(), db, func(ctx context.Context) error {
		return Transact(ctx, db, func(innerCtx context.Context) error {
			innerTx, err := ctxval.GetPrimaryTransaction(innerCtx)
			require.NoError(t, err)
			assert.Equal(t, tx, innerTx)
			return nil
		})
	})

	require.NoError(t, err)
	assert.True(t, tx.committed)
	assert.False(t, tx.rolledBack)
}

func TestDefaultQueryExecMode_Configuration(t *testing.T) {
	// Without query parameter, DefaultQueryExecMode defaults to QueryExecModeCacheStatement
	cfg, err := pgxpool.ParseConfig("postgres://user:pass@localhost:5432/dbname?sslmode=disable")
	require.NoError(t, err)
	assert.Equal(t, pgx.QueryExecModeCacheStatement, cfg.ConnConfig.DefaultQueryExecMode)

	// If defaulted to CacheStatement, we override to QueryExecModeExec
	if cfg.ConnConfig.DefaultQueryExecMode == pgx.QueryExecModeCacheStatement {
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	}
	assert.Equal(t, pgx.QueryExecModeExec, cfg.ConnConfig.DefaultQueryExecMode)

	// If explicitly set in URL, it preserves the user choice
	cfgExplicit, err := pgxpool.ParseConfig("postgres://user:pass@localhost:5432/dbname?sslmode=disable&default_query_exec_mode=simple_protocol")
	require.NoError(t, err)
	assert.Equal(t, pgx.QueryExecModeSimpleProtocol, cfgExplicit.ConnConfig.DefaultQueryExecMode)
	if cfgExplicit.ConnConfig.DefaultQueryExecMode == pgx.QueryExecModeCacheStatement {
		cfgExplicit.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	}
	assert.Equal(t, pgx.QueryExecModeSimpleProtocol, cfgExplicit.ConnConfig.DefaultQueryExecMode)
}
