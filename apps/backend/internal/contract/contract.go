package contract

import (
	"context"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/common"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/usecase"
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

// New assembles the contract from configuration only. It does not open the
// database connection — that happens separately (in the delivery layer) so
// contract wiring and DB initialization remain independent concerns.
// The returned stopper closes any resources the contract owns.
func New(cfg *config.Config) (*Contract, Stopper, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("config is required")
	}

	c := &Contract{
		Cfg:        cfg,
		Usecase:    &usecase.Contract{},
		Repository: &repository.Contract{},
		Common:     &common.Contract{},
	}

	// Contract itself owns no closable resources yet; the DB stopper comes
	// from common.New and is composed by the delivery layer.
	var stopper Stopper = func(ctx context.Context) error {
		if c.Common != nil && c.Common.DB != nil {
			c.Common.DB.Close()
		}
		return nil
	}

	return c, stopper, nil
}
