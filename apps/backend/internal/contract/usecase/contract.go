package usecase

import (
	"errors"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
)

// Contract is the usecase layer contract. Usecase implementations attach
// their business-logic methods here as the domain model grows.
type Contract struct {
	User ucUsers.User
}

// NewUsecase creates the usecase layer contract from the repository contract.
// It instantiates concrete usecase types backed by the shared repository contract.
func NewUsecase(r *repository.Contract) (*Contract, error) {
	if r == nil || r.UsersRepository == nil {
		return nil, errors.New("repository contract with UsersRepository is required")
	}

	c := &Contract{
		User: ucUsers.NewUserService(r.UsersRepository),
	}
	return c, nil
}
