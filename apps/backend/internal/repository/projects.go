package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// Projects is the interface for user_projects data access.
type Projects interface {
	Create(ctx context.Context, params models.CreateProjectParams) (*models.Project, error)
	GetByID(ctx context.Context, params models.GetProjectParams) (*models.Project, error)
	ListByUser(ctx context.Context, userID string) ([]*models.Project, error)
	Update(ctx context.Context, params models.UpdateProjectParams) (*models.Project, error)
}

type projectsRepository struct {
	db database.DB
}

// NewProjectsRepository creates a Projects backed by the given database.
func NewProjectsRepository(db database.DB) Projects {
	return &projectsRepository{db: db}
}

func (r *projectsRepository) Create(ctx context.Context, params models.CreateProjectParams) (*models.Project, error) {
	id := uuid.NewString()

	query := `
		INSERT INTO user_projects (
			id, user_id, user_experience_id, name, description, responsibilities,
			achievements, started_at, ended_at, is_current
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, user_id, user_experience_id, name, description, responsibilities,
			achievements, started_at, ended_at, is_current, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, query, id, params.UserID, params.UserExperienceID, params.Name,
		params.Description, params.Responsibilities, params.Achievements,
		params.StartedAt, params.EndedAt, params.IsCurrent)

	var p models.Project
	var uxID sql.NullString
	var desc, resp, ach sql.NullString
	var started, ended, created, updated sql.NullTime
	if err := row.Scan(
		&p.ID, &p.UserID, &uxID, &p.Name, &desc, &resp,
		&ach, &started, &ended, &p.IsCurrent, &created, &updated,
	); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	p.UserExperienceID = utils.NullString(uxID)
	p.Description = utils.NullString(desc)
	p.Responsibilities = utils.NullString(resp)
	p.Achievements = utils.NullString(ach)
	p.StartedAt = utils.NullTime(started)
	p.EndedAt = utils.NullTime(ended)
	p.CreatedAt = utils.NullTime(created)
	p.UpdatedAt = utils.NullTime(updated)
	return &p, nil
}

func (r *projectsRepository) GetByID(ctx context.Context, params models.GetProjectParams) (*models.Project, error) {
	query := `
		SELECT id, user_id, user_experience_id, name, description, responsibilities,
			achievements, started_at, ended_at, is_current, created_at, updated_at
		FROM user_projects
		WHERE id = $1 AND user_id = $2
	`
	row := r.db.QueryRow(ctx, query, params.ID, params.UserID)

	var p models.Project
	var uxID sql.NullString
	var desc, resp, ach sql.NullString
	var started, ended, created, updated sql.NullTime
	if err := row.Scan(
		&p.ID, &p.UserID, &uxID, &p.Name, &desc, &resp,
		&ach, &started, &ended, &p.IsCurrent, &created, &updated,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	p.UserExperienceID = utils.NullString(uxID)
	p.Description = utils.NullString(desc)
	p.Responsibilities = utils.NullString(resp)
	p.Achievements = utils.NullString(ach)
	p.StartedAt = utils.NullTime(started)
	p.EndedAt = utils.NullTime(ended)
	p.CreatedAt = utils.NullTime(created)
	p.UpdatedAt = utils.NullTime(updated)
	return &p, nil
}

func (r *projectsRepository) ListByUser(ctx context.Context, userID string) ([]*models.Project, error) {
	query := `
		SELECT id, user_id, user_experience_id, name, description, responsibilities,
			achievements, started_at, ended_at, is_current, created_at, updated_at
		FROM user_projects
		WHERE user_id = $1
		ORDER BY started_at DESC NULLS LAST
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		var p models.Project
		var uxID sql.NullString
		var desc, resp, ach sql.NullString
		var started, ended, created, updated sql.NullTime
		if err := rows.Scan(
			&p.ID, &p.UserID, &uxID, &p.Name, &desc, &resp,
			&ach, &started, &ended, &p.IsCurrent, &created, &updated,
		); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.UserExperienceID = utils.NullString(uxID)
		p.Description = utils.NullString(desc)
		p.Responsibilities = utils.NullString(resp)
		p.Achievements = utils.NullString(ach)
		p.StartedAt = utils.NullTime(started)
		p.EndedAt = utils.NullTime(ended)
		p.CreatedAt = utils.NullTime(created)
		p.UpdatedAt = utils.NullTime(updated)
		projects = append(projects, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return projects, nil
}

func (r *projectsRepository) Update(ctx context.Context, params models.UpdateProjectParams) (*models.Project, error) {
	query := `
		UPDATE user_projects
		SET user_experience_id = $3, name = $4, description = $5, responsibilities = $6,
			achievements = $7, started_at = $8, ended_at = $9, is_current = $10,
			updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, user_experience_id, name, description, responsibilities,
			achievements, started_at, ended_at, is_current, created_at, updated_at
	`
	row := r.db.QueryRow(ctx, query, params.ID, params.UserID, params.UserExperienceID, params.Name,
		params.Description, params.Responsibilities, params.Achievements,
		params.StartedAt, params.EndedAt, params.IsCurrent)

	var p models.Project
	var uxID sql.NullString
	var desc, resp, ach sql.NullString
	var started, ended, created, updated sql.NullTime
	if err := row.Scan(
		&p.ID, &p.UserID, &uxID, &p.Name, &desc, &resp,
		&ach, &started, &ended, &p.IsCurrent, &created, &updated,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("update project: %w", err)
	}
	p.UserExperienceID = utils.NullString(uxID)
	p.Description = utils.NullString(desc)
	p.Responsibilities = utils.NullString(resp)
	p.Achievements = utils.NullString(ach)
	p.StartedAt = utils.NullTime(started)
	p.EndedAt = utils.NullTime(ended)
	p.CreatedAt = utils.NullTime(created)
	p.UpdatedAt = utils.NullTime(updated)
	return &p, nil
}
