package repository

import (
	"context"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// UserPreferences is the interface for accessing user preferences.
type UserPreferences interface {
	GetActivePreferences(ctx context.Context, userIDs []string) ([]models.UserPreference, error)
}

type userPreferencesRepository struct {
	*database.PG
}

// NewUserPreferencesRepository creates a new UserPreferences repository backed by PG.
func NewUserPreferencesRepository(pg *database.PG) UserPreferences {
	return &userPreferencesRepository{
		PG: pg,
	}
}

// GetActivePreferences retrieves active preferences. If userIDs is empty,
// all active preferences are returned; otherwise, only preferences matching
// the given userIDs are returned.
func (r *userPreferencesRepository) GetActivePreferences(ctx context.Context, userIDs []string) ([]models.UserPreference, error) {
	var query string
	var args []any

	if len(userIDs) > 0 {
		query = `
			SELECT id, user_id, keywords, locations, boards, is_active, created_at, updated_at
			FROM user_preferences
			WHERE is_active = TRUE AND user_id = ANY($1)
			ORDER BY id ASC
		`
		args = append(args, userIDs)
	} else {
		query = `
			SELECT id, user_id, keywords, locations, boards, is_active, created_at, updated_at
			FROM user_preferences
			WHERE is_active = TRUE
			ORDER BY id ASC
		`
	}

	rows, err := r.DB().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query user_preferences: %w", err)
	}
	defer rows.Close()

	var preferences []models.UserPreference
	for rows.Next() {
		var p models.UserPreference
		if err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Keywords,
			&p.Locations,
			&p.Boards,
			&p.IsActive,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user_preference: %w", err)
		}
		preferences = append(preferences, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows user_preferences: %w", err)
	}

	return preferences, nil
}
