package database

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/graceful"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is the database connection abstraction. Any provider (Neon, Supabase,
// self-managed Postgres, ...) that implements Ping, Close, QueryRow, and Query
// can be plugged in without changing the delivery, contract, or repository
// layers.
type DB interface {
	Ping(ctx context.Context) error
	Close()
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
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
