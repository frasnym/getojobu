package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// User resolves Clerk identities to local application users.
// Users are created exclusively via Clerk webhook, never on first request.
type User interface {
	FindByClerkUserID(ctx context.Context, clerkUserID string) (*models.Principal, error)
	FindByID(ctx context.Context, id string) (*models.User, error)
	Create(ctx context.Context, clerkUserID string, email, fullName string, role models.Role) (*models.User, error)
	Update(ctx context.Context, id string, email, fullName string) (*models.User, error)
	SoftDelete(ctx context.Context, id string) (*models.User, error)
}

// userService resolves Clerk identities to local users.
// Users are created exclusively via Clerk webhook, never on first request.
type userService struct {
	repo repository.Users
}

// NewUserService creates a user service backed by a repository.
func NewUserService(repo repository.Users) User {
	return &userService{repo: repo}
}

// FindByClerkUserID finds an existing local user by Clerk user ID.
// Returns ErrUserNotFound when no local record exists — users must be
// created via Clerk webhook before they can authenticate.
func (s *userService) FindByClerkUserID(ctx context.Context, clerkUserID string) (*models.Principal, error) {
	u, err := s.repo.FindByClerkUserID(ctx, clerkUserID)
	if err != nil {
		if errors.Is(err, constants.ErrUserNotFound) {
			return nil, constants.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}

	// Existing user: check deactivation.
	if u.DeletedAt != nil {
		return nil, constants.ErrAccountDeactivated
	}

	return models.NewPrincipalFromUser(u), nil
}

// FindByID returns the local user record with the given internal user id.
func (s *userService) FindByID(ctx context.Context, id string) (*models.User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, constants.ErrUserNotFound) {
			return nil, constants.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

// Create creates a new local user via Clerk webhook.
func (s *userService) Create(ctx context.Context, clerkUserID string, email, fullName string, role models.Role) (*models.User, error) {
	return s.repo.Create(ctx, clerkUserID, email, fullName, role)
}

// Update modifies an existing local user's email and full name.
func (s *userService) Update(ctx context.Context, id string, email, fullName string) (*models.User, error) {
	return s.repo.Update(ctx, id, email, fullName)
}

// SoftDelete marks a local user as deactivated by setting deleted_at.
func (s *userService) SoftDelete(ctx context.Context, id string) (*models.User, error) {
	return s.repo.SoftDelete(ctx, id)
}
