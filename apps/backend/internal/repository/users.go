package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// UsersRepository finds and creates local user records.
type UsersRepository struct {
	db database.DB
}

// NewUsersRepository creates a UsersRepository backed by the given database.
func NewUsersRepository(db database.DB) *UsersRepository {
	return &UsersRepository{db: db}
}

// FindByClerkUserID returns the local user for the given Clerk user ID,
// or sql.ErrNoRows when no local record exists.
func (r *UsersRepository) FindByClerkUserID(ctx context.Context, clerkUserID string) (*models.User, error) {
	query := `
		SELECT id, clerk_user_id, email, full_name, role, onboarding_completed_at, deleted_at
		FROM users
		WHERE clerk_user_id = $1
	`
	row := r.db.QueryRow(ctx, query, clerkUserID)

	var u models.User
	if err := row.Scan(
		&u.ID,
		&u.ClerkUserID,
		&u.Email,
		&u.FullName,
		&u.Role,
		&u.OnboardingCompletedAt,
		&u.DeletedAt,
	); err != nil {
		return nil, fmt.Errorf("find user by clerk id: %w", err)
	}
	return &u, nil
}

// Create inserts a new local user and returns the created record.
// The ID is generated in Go code (not via gen_random_uuid) and role is set
// explicitly by the caller (e.g. models.RoleUser); no SQL default is used.
func (r *UsersRepository) Create(ctx context.Context, clerkUserID string, email, fullName string, role models.Role) (*models.User, error) {
	id := uuid.NewString()

	query := `
		INSERT INTO users (id, clerk_user_id, email, full_name, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, clerk_user_id, email, full_name, role, onboarding_completed_at, deleted_at
	`
	row := r.db.QueryRow(ctx, query, id, clerkUserID, email, fullName, role)

	var u models.User
	if err := row.Scan(
		&u.ID,
		&u.ClerkUserID,
		&u.Email,
		&u.FullName,
		&u.Role,
		&u.OnboardingCompletedAt,
		&u.DeletedAt,
	); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

// Update modifies an existing local user's email and full name.
// Used by Clerk webhook when a user record is updated in Clerk.
func (r *UsersRepository) Update(ctx context.Context, id string, email, fullName string) (*models.User, error) {
	query := `
		UPDATE users
		SET email = $2, full_name = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id, clerk_user_id, email, full_name, role, onboarding_completed_at, deleted_at
	`
	row := r.db.QueryRow(ctx, query, id, email, fullName)

	var u models.User
	if err := row.Scan(
		&u.ID,
		&u.ClerkUserID,
		&u.Email,
		&u.FullName,
		&u.Role,
		&u.OnboardingCompletedAt,
		&u.DeletedAt,
	); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return &u, nil
}

// SoftDelete marks a local user as deactivated by setting deleted_at.
// Used by Clerk webhook when a user is deleted in Clerk.
// Does not remove the row, preserving referential integrity with any
// application or audit history.
func (r *UsersRepository) SoftDelete(ctx context.Context, id string) (*models.User, error) {
	query := `
		UPDATE users
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1
		RETURNING id, clerk_user_id, email, full_name, role, onboarding_completed_at, deleted_at
	`
	row := r.db.QueryRow(ctx, query, id)

	var u models.User
	if err := row.Scan(
		&u.ID,
		&u.ClerkUserID,
		&u.Email,
		&u.FullName,
		&u.Role,
		&u.OnboardingCompletedAt,
		&u.DeletedAt,
	); err != nil {
		return nil, fmt.Errorf("soft delete user: %w", err)
	}
	return &u, nil
}
