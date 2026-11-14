package common

import (
	"context"
	"errors"
	"fmt"

	"go.temporal.io/sdk/client"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	repo "github.com/frasnym/getojobu/apps/backend/internal/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/graceful"
)

// Contract holds shared infrastructure dependencies, currently the database
// connection, Temporal orchestrator client, and object storage client.
// Placing them here keeps repository and usecase layers free of infrastructure concerns.
type Contract struct {
	DB                database.DB
	Temporal          client.Client
	TemporalTaskQueue string
	Storage           repo.Storage
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

	var temporalClient client.Client
	if cfg.Temporal.Host != "" {
		tc, err := client.NewLazyClient(client.Options{
			HostPort:  cfg.Temporal.Host,
			Namespace: cfg.Temporal.Namespace,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create temporal client: %w", err)
		}
		temporalClient = tc
		stoppers = append(stoppers, func(ctx context.Context) error {
			tc.Close()
			return nil
		})
	}
	contract.Temporal = temporalClient
	contract.TemporalTaskQueue = cfg.Temporal.TaskQueue
	contract.Storage = repo.NewR2StorageRepository(cfg.R2)

	return contract, stopper, nil
}
