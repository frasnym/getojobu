package database

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/graceful"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is the database connection abstraction. Any provider (Neon, Supabase,
// self-managed Postgres, ...) that implements Ping, Close, QueryRow, Query,
// and Begin can be plugged in without changing the delivery, contract, or
// repository layers.
//
// Begin returns pgx.Tx directly because pgx.Pool.Begin(ctx) returns pgx.Tx.
// pgx is already a dependency of this package (pgx.Row, pgx.Rows), so this
// adds no new coupling.
type DB interface {
	Ping(ctx context.Context) error
	Close()
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

// NewDB creates a connection to a PostgreSQL database using config and returns
// the connection plus a stopper that closes it. It reads cfg.DB.URL first;
// when unset, it constructs a connection URL from cfg.DB's PG* fields
// (Host, Port, Database, User, Password, SSLMode, ChannelBinding). This works
// with any PostgreSQL-compatible provider without changing code.
func NewDB(cfg *config.Config) (DB, graceful.ProcessStopper, error) {
	if cfg == nil {
		return nil, nil, errors.New("config is required")
	}

	connectionURL := cfg.DB.URL
	if connectionURL == "" {
		connectionURL = buildURLFromPGEnv(cfg.DB)
	}
	if connectionURL == "" {
		return nil, nil, errors.New("database URL is required (set DATABASE_URL or PG_HOST/PG_PORT/PG_DATABASE/PG_USER/PG_PASSWORD)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connectionURL)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to database: %w", err)
	}

	// Ping eagerly so a misconfigured or unreachable database fails fast at
	// startup instead of surfacing later through a 503 on the first request.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("ping database: %w", err)
	}

	stopper := func(ctx context.Context) error {
		if pool != nil {
			pool.Close()
		}
		return nil
	}

	var db DB = pool
	return db, stopper, nil
}

// Transact executes operation inside a transaction.
// If ctx already has an active transaction (from an outer Transact call),
// it reuses it without beginning a new one (nested/child transaction).
// Otherwise, it begins a new transaction on db, binds it to ctx,
// and commits on success or rolls back on error/panic.
//
// This mirrors go-ewallet-core's autotx.Transact: the caller supplies one
// operation closure func(ctx context.Context) error that does all the work
// inside the transaction, and begin/commit/rollback is centralized here.
func Transact(ctx context.Context, db DB, operation func(ctx context.Context) error) (err error) {
	var (
		child = true
		tx    pgx.Tx
	)
	_, err = ctxval.GetPrimaryTransaction(ctx)
	if err != nil {
		tx, err = db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		ctx = ctxval.SetPrimaryTransaction(ctx, tx)
		child = false
	}

	defer func() {
		if !child {
			if p := recover(); p != nil {
				_ = tx.Rollback(ctx)
				panic(p)
			} else if err != nil {
				if rerr := tx.Rollback(ctx); rerr != nil {
					err = fmt.Errorf("rollback failed: %w; original: %w", rerr, err)
				}
			} else {
				if cerr := tx.Commit(ctx); cerr != nil {
					err = fmt.Errorf("commit: %w", cerr)
				}
			}
		}
	}()
	err = operation(ctx)
	return err
}

// buildURLFromPGEnv constructs a PostgreSQL connection URL from PG* fields.
func buildURLFromPGEnv(db config.DBConfig) string {
	host := db.Host
	database := db.Database
	user := db.User
	password := db.Password
	sslmode := db.SSLMode
	channelBinding := db.ChannelBinding
	port := db.Port

	if host == "" && database == "" && user == "" && password == "" && sslmode == "" && channelBinding == "" && port == "" {
		return ""
	}

	u := &url.URL{
		Scheme: "postgresql",
	}

	if host != "" {
		if port != "" {
			u.Host = net.JoinHostPort(host, port)
		} else {
			u.Host = host
		}
	}

	if database != "" {
		u.Path = "/" + database
	}

	if user != "" {
		u.User = url.User(user)
		if password != "" {
			u.User = url.UserPassword(user, password)
		}
	}

	q := url.Values{}
	if sslmode != "" {
		q.Set("sslmode", sslmode)
	}
	if channelBinding != "" {
		q.Set("channel_binding", channelBinding)
	}
	if len(q) > 0 {
		u.RawQuery = q.Encode()
	}

	return u.String()
}
