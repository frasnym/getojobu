package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
)

// Transactor coordinates atomic operations inside a transaction.
type Transactor interface {
	Transact(ctx context.Context, operation func(ctx context.Context) error) error
}

// PG provides centralized transaction management for repository implementations.
type PG struct {
	db DB
}

// NewPG creates a new PG transaction manager backed by the given database.
func NewPG(db DB) *PG {
	return &PG{db: db}
}

// DB returns the underlying DB instance.
func (m *PG) DB() DB {
	return m.db
}

// Transact executes operation inside a transaction using the underlying database.
func (m *PG) Transact(ctx context.Context, operation func(ctx context.Context) error) error {
	return Transact(ctx, m.db, operation)
}

// Committer handles commit & rollback for non-context transactions.
// If an active transaction was already present in ctx (e.g. from an outer
// Transact / operation call), Committer does nothing so the outer caller can
// commit or rollback the entire unit of work.
func (m *PG) Committer(ctx context.Context, trx pgx.Tx, err *error) {
	_, errCtx := ctxval.GetPrimaryTransaction(ctx)
	if errCtx != nil && trx != nil {
		if p := recover(); p != nil {
			_ = trx.Rollback(ctx)
			panic(p)
		} else if *err != nil {
			_ = trx.Rollback(ctx)
			return
		}
		*err = trx.Commit(ctx)
	}
}

// StartTrx starts a transaction based on context if exists, or creates a new
// transaction if not exists. Always defer Committer(ctx, trx, &err) when calling StartTrx.
func (m *PG) StartTrx(ctx context.Context) (trx pgx.Tx, err error) {
	trx, err = ctxval.GetPrimaryTransaction(ctx)
	if err != nil {
		if errors.Is(err, constants.ErrNoActiveConnections) {
			trx, err = m.db.Begin(ctx)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return trx, nil
}

// QueryRow executes a query returning at most one row. It uses the active transaction
// stored in ctx if present; otherwise it executes directly against the database pool.
func (m *PG) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, err := ctxval.GetPrimaryTransaction(ctx); err == nil && tx != nil {
		return tx.QueryRow(ctx, sql, args...)
	}
	return m.db.QueryRow(ctx, sql, args...)
}

// Query executes a query returning rows. It uses the active transaction stored in
// ctx if present; otherwise it executes directly against the database pool.
func (m *PG) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx, err := ctxval.GetPrimaryTransaction(ctx); err == nil && tx != nil {
		return tx.Query(ctx, sql, args...)
	}
	return m.db.Query(ctx, sql, args...)
}
