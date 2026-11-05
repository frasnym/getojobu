package common

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/graceful"
)

// Contract holds shared infrastructure dependencies, currently the database
// connection. Placing DB here keeps repository and usecase layers free of
// infrastructure concerns. DB is an interface so any provider (Neon,
// Supabase, self-managed Postgres) can be swapped without changing this layer.
type Contract struct {
	DB database.DB
}

// New creates a connection to a PostgreSQL database using config and returns
// the contract plus its own stopper. Database initialization is intentionally
// separate from contract wiring. Reads cfg.DB.URL first; when unset, constructs
// a URL from cfg.DB's PG* fields. This works with any PostgreSQL-compatible
// provider without changing code.
func New(cfg *config.Config) (*Contract, graceful.ProcessStopper, error) {
	contract := &Contract{}
	stoppers := []graceful.ProcessStopper{}
	stopper := func(ctx context.Context) error {
		// Run every stopper even when one fails, and report them together.
		// Returning on the first error stranded everything registered behind it,
		// which here is the database connection: the one resource shutdown
		// exists to release.
		var errs []error
		for _, st := range stoppers {
			if st == nil {
				continue
			}
			if err := st(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}

	if cfg == nil {
		return nil, nil, errors.New("config is required")
	}

	db, stopper, err := database.NewDB(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create database connection: %w", err)
	}
	stoppers = append(stoppers, stopper)
	contract.DB = db

	return contract, stopper, nil
}

// Close releases the database connection.
func (c *Contract) Close() {
	if c != nil && c.DB != nil {
		c.DB.Close()
	}
}

// Health checks that the database is reachable within a short deadline.
func (c *Contract) Health(ctx context.Context) error {
	if c == nil || c.DB == nil {
		return errors.New("database not initialized")
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return c.DB.Ping(ctx)
}
