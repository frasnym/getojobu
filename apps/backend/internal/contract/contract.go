package contract

import (
	"context"
	"errors"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/common"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/usecase"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/graceful"
)

// Stopper gracefully shuts down a resource within a context deadline.
type Stopper func(context.Context) error

// Contract aggregates all layer contracts into a single dependency container,
// mirroring the wiring pattern used by Amartha services (e.g. go-ap-ppob).
type Contract struct {
	Cfg        *config.Config
	Usecase    *usecase.Contract
	Repository *repository.Contract
	Common     *common.Contract
}

// New assembles the contract from configuration and initializes the common
// layer (database pool) fail-fast: if the database is unreachable, New returns
// an error and no contract. The returned stopper closes all registered
// resources, running the database stopper last so pools stay open until
// nothing uses them.
func New(cfg *config.Config) (*Contract, Stopper, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("config is required")
	}

	contract := &Contract{Cfg: cfg}
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

	// Initialize the common layer (DB pool, etc.) first, so that the rest of
	// the contract can use it. The stopper is registered to run last in the
	// shutdown sequence, so that the DB pool stays open until nothing uses it.
	commonContract, commonStopper, err := common.New(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize common layer: %w", err)
	}
	contract.Common = commonContract
	stoppers = append(stoppers, commonStopper)

	return contract, stopper, nil
}
