package repository

import (
	"errors"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/common"
	repo "github.com/frasnym/getojobu/apps/backend/internal/repository"
)

// Contract is the repository layer contract. Repository implementations
// attach their data-access methods here as the domain model grows.
type Contract struct {
	UsersRepository *repo.UsersRepository
}

// NewRepository creates the repository layer contract from the common
// infrastructure contract. It instantiates concrete repository types backed
// by the shared database connection.
func NewRepository(comm *common.Contract) (*Contract, error) {
	if comm == nil || comm.DB == nil {
		return nil, errors.New("common database is required")
	}

	c := &Contract{
		UsersRepository: repo.NewUsersRepository(comm.DB),
	}
	return c, nil
}
