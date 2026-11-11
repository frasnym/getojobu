package database

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
)

func TestPG_StartTrx_and_Committer_standalone_success(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}
	p := NewPG(db)
	assert.Equal(t, db, p.DB())

	ctx := context.Background()
	run := func() (err error) {
		trx, err := p.StartTrx(ctx)
		if err != nil {
			return err
		}
		defer p.Committer(ctx, trx, &err)
		return nil
	}

	err := run()
	require.NoError(t, err)
	assert.True(t, tx.committed)
	assert.False(t, tx.rolledBack)
}

func TestPG_StartTrx_and_Committer_standalone_error(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}
	p := NewPG(db)

	ctx := context.Background()
	failErr := errors.New("query failed")
	run := func() (err error) {
		trx, err := p.StartTrx(ctx)
		if err != nil {
			return err
		}
		defer p.Committer(ctx, trx, &err)
		return failErr
	}

	err := run()
	require.ErrorIs(t, err, failErr)
	assert.False(t, tx.committed)
	assert.True(t, tx.rolledBack)
}

func TestPG_StartTrx_and_Committer_context_delegates_to_parent(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}
	p := NewPG(db)

	ctx := ctxval.SetPrimaryTransaction(context.Background(), tx)

	run := func() (err error) {
		trx, err := p.StartTrx(ctx)
		if err != nil {
			return err
		}
		defer p.Committer(ctx, trx, &err)
		assert.Equal(t, tx, trx)
		return nil
	}

	err := run()
	require.NoError(t, err)
	// Because tx was in context, Committer does not commit or rollback
	assert.False(t, tx.committed)
	assert.False(t, tx.rolledBack)
}

func TestPG_StartTrx_and_Transact_end_to_end(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}
	p := NewPG(db)

	err := Transact(context.Background(), db, func(ctx context.Context) (err error) {
		trx, err := p.StartTrx(ctx)
		if err != nil {
			return err
		}
		defer p.Committer(ctx, trx, &err)

		assert.Equal(t, tx, trx)
		return nil
	})

	require.NoError(t, err)
	// Committed by Transact
	assert.True(t, tx.committed)
	assert.False(t, tx.rolledBack)
}

func TestPG_Transact_method(t *testing.T) {
	tx := &mockTx{}
	db := &mockDB{tx: tx}
	p := NewPG(db)

	var ran bool
	err := p.Transact(context.Background(), func(ctx context.Context) error {
		ran = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, ran)
	assert.True(t, tx.committed)
}

func TestPG_QueryRow_and_Query_delegation(t *testing.T) {
	t.Run("without active tx delegates to db", func(t *testing.T) {
		db := &mockDB{}
		p := NewPG(db)

		_ = p.QueryRow(context.Background(), "SELECT 1")
		_, _ = p.Query(context.Background(), "SELECT 1")
	})

	t.Run("with active tx delegates to tx", func(t *testing.T) {
		tx := &mockTx{}
		db := &mockDB{tx: tx}
		p := NewPG(db)

		ctx := ctxval.SetPrimaryTransaction(context.Background(), tx)
		_ = p.QueryRow(ctx, "SELECT 1")
		_, _ = p.Query(ctx, "SELECT 1")
	})
}
