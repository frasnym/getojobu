package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// Experiences is the interface for user_experiences data access.
type Experiences interface {
	Create(ctx context.Context, params models.CreateExperienceParams) (*models.Experience, error)
	GetByID(ctx context.Context, params models.GetExperienceParams) (*models.Experience, error)
	ListByUser(ctx context.Context, userID string) ([]*models.Experience, error)
	Update(ctx context.Context, params models.UpdateExperienceParams) (*models.Experience, error)
}

type experiencesRepository struct {
	db database.DB
}

// NewExperiencesRepository creates an Experiences backed by the given database.
func NewExperiencesRepository(db database.DB) Experiences {
	return &experiencesRepository{db: db}
}

func (r *experiencesRepository) Create(ctx context.Context, params models.CreateExperienceParams) (*models.Experience, error) {
	if !params.ExperienceType.IsValid() {
		return nil, fmt.Errorf("create experience: invalid experience_type %q", params.ExperienceType)
	}

	id := uuid.NewString()

	query := `
		INSERT INTO user_experiences (
			id, user_id, experience_type, organization, role_title, location,
			description, started_at, ended_at, is_current
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, user_id, experience_type, organization, role_title, location,
			description, started_at, ended_at, is_current, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, query, id, params.UserID, params.ExperienceType, params.Organization,
		params.RoleTitle, params.Location, params.Description, params.StartedAt, params.EndedAt, params.IsCurrent)

	var e models.Experience
	var org, role, loc, desc sql.NullString
	var started, ended, created, updated sql.NullTime
	if err := row.Scan(
		&e.ID, &e.UserID, &e.ExperienceType, &org, &role, &loc,
		&desc, &started, &ended, &e.IsCurrent, &created, &updated,
	); err != nil {
		return nil, fmt.Errorf("create experience: %w", err)
	}
	e.Organization = utils.NullString(org)
	e.RoleTitle = utils.NullString(role)
	e.Location = utils.NullString(loc)
	e.Description = utils.NullString(desc)
	e.StartedAt = utils.NullTime(started)
	e.EndedAt = utils.NullTime(ended)
	e.CreatedAt = utils.NullTime(created)
	e.UpdatedAt = utils.NullTime(updated)
	return &e, nil
}

func (r *experiencesRepository) GetByID(ctx context.Context, params models.GetExperienceParams) (*models.Experience, error) {
	query := `
		SELECT id, user_id, experience_type, organization, role_title, location,
			description, started_at, ended_at, is_current, created_at, updated_at
		FROM user_experiences
		WHERE id = $1 AND user_id = $2
	`
	row := r.db.QueryRow(ctx, query, params.ID, params.UserID)

	var e models.Experience
	var org, role, loc, desc sql.NullString
	var started, ended, created, updated sql.NullTime
	if err := row.Scan(
		&e.ID, &e.UserID, &e.ExperienceType, &org, &role, &loc,
		&desc, &started, &ended, &e.IsCurrent, &created, &updated,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("get experience: %w", err)
	}
	e.Organization = utils.NullString(org)
	e.RoleTitle = utils.NullString(role)
	e.Location = utils.NullString(loc)
	e.Description = utils.NullString(desc)
	e.StartedAt = utils.NullTime(started)
	e.EndedAt = utils.NullTime(ended)
	e.CreatedAt = utils.NullTime(created)
	e.UpdatedAt = utils.NullTime(updated)
	return &e, nil
}

func (r *experiencesRepository) ListByUser(ctx context.Context, userID string) ([]*models.Experience, error) {
	query := `
		SELECT id, user_id, experience_type, organization, role_title, location,
			description, started_at, ended_at, is_current, created_at, updated_at
		FROM user_experiences
		WHERE user_id = $1
		ORDER BY started_at DESC NULLS LAST
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list experiences: %w", err)
	}
	defer rows.Close()

	var experiences []*models.Experience
	for rows.Next() {
		var e models.Experience
		var org, role, loc, desc sql.NullString
		var started, ended, created, updated sql.NullTime
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.ExperienceType, &org, &role, &loc,
			&desc, &started, &ended, &e.IsCurrent, &created, &updated,
		); err != nil {
			return nil, fmt.Errorf("scan experience: %w", err)
		}
		e.Organization = utils.NullString(org)
		e.RoleTitle = utils.NullString(role)
		e.Location = utils.NullString(loc)
		e.Description = utils.NullString(desc)
		e.StartedAt = utils.NullTime(started)
		e.EndedAt = utils.NullTime(ended)
		e.CreatedAt = utils.NullTime(created)
		e.UpdatedAt = utils.NullTime(updated)
		experiences = append(experiences, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experiences: %w", err)
	}
	return experiences, nil
}

func (r *experiencesRepository) Update(ctx context.Context, params models.UpdateExperienceParams) (*models.Experience, error) {
	if !params.ExperienceType.IsValid() {
		return nil, fmt.Errorf("update experience: invalid experience_type %q", params.ExperienceType)
	}

	query := `
		UPDATE user_experiences
		SET experience_type = $3, organization = $4, role_title = $5, location = $6,
			description = $7, started_at = $8, ended_at = $9, is_current = $10,
			updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, experience_type, organization, role_title, location,
			description, started_at, ended_at, is_current, created_at, updated_at
	`
	row := r.db.QueryRow(ctx, query, params.ID, params.UserID, params.ExperienceType, params.Organization,
		params.RoleTitle, params.Location, params.Description, params.StartedAt, params.EndedAt, params.IsCurrent)

	var e models.Experience
	var org, role, loc, desc sql.NullString
	var started, ended, created, updated sql.NullTime
	if err := row.Scan(
		&e.ID, &e.UserID, &e.ExperienceType, &org, &role, &loc,
		&desc, &started, &ended, &e.IsCurrent, &created, &updated,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("update experience: %w", err)
	}
	e.Organization = utils.NullString(org)
	e.RoleTitle = utils.NullString(role)
	e.Location = utils.NullString(loc)
	e.Description = utils.NullString(desc)
	e.StartedAt = utils.NullTime(started)
	e.EndedAt = utils.NullTime(ended)
	e.CreatedAt = utils.NullTime(created)
	e.UpdatedAt = utils.NullTime(updated)
	return &e, nil
}
