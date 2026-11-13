package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// ProjectSkills is the interface for user_project_skills data access.
type ProjectSkills interface {
	LinkSkills(ctx context.Context, params models.LinkProjectSkillsParams) error
	ListSkillsForProject(ctx context.Context, params models.ListProjectSkillsParams) ([]*models.UserSkill, error)
	ListSkillsByUser(ctx context.Context, userID string) ([]*models.UserSkill, error)
}

type projectSkillsRepository struct {
	db database.DB
}

// NewProjectSkillsRepository creates a ProjectSkills backed by the given database.
func NewProjectSkillsRepository(db database.DB) ProjectSkills {
	return &projectSkillsRepository{db: db}
}

func (r *projectSkillsRepository) LinkSkills(ctx context.Context, params models.LinkProjectSkillsParams) error {
	for _, skillID := range params.SkillIDs {
		query := `
			INSERT INTO user_project_skills (user_id, user_project_id, user_skill_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_project_id, user_skill_id) DO NOTHING
		`
		row := r.db.QueryRow(ctx, query, params.UserID, params.ProjectID, skillID)
		var dummy string
		if err := row.Scan(&dummy); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("link project skill: %w", err)
		}
	}
	return nil
}

func (r *projectSkillsRepository) ListSkillsForProject(ctx context.Context, params models.ListProjectSkillsParams) ([]*models.UserSkill, error) {
	query := `
		SELECT s.id, s.user_id, s.skill_name, s.skill_key, s.proficiency, s.created_at, s.updated_at
		FROM user_project_skills ps
		JOIN user_skills s
			ON s.id = ps.user_skill_id
		   AND s.user_id = ps.user_id
		WHERE ps.user_id = $1
		  AND ps.user_project_id = $2
		ORDER BY s.skill_name
	`
	rows, err := r.db.Query(ctx, query, params.UserID, params.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("list project skills: %w", err)
	}
	defer rows.Close()

	var skills []*models.UserSkill
	for rows.Next() {
		var s models.UserSkill
		var prof sql.NullString
		var created, updated sql.NullTime
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.SkillName, &s.SkillKey, &prof, &created, &updated,
		); err != nil {
			return nil, fmt.Errorf("scan project skill: %w", err)
		}
		s.Proficiency = utils.NullString(prof)
		s.CreatedAt = utils.NullTime(created)
		s.UpdatedAt = utils.NullTime(updated)
		skills = append(skills, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project skills: %w", err)
	}
	return skills, nil
}

func (r *projectSkillsRepository) ListSkillsByUser(ctx context.Context, userID string) ([]*models.UserSkill, error) {
	query := `
		SELECT id, user_id, skill_name, skill_key, proficiency, created_at, updated_at
		FROM user_skills
		WHERE user_id = $1
		ORDER BY skill_name
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list user skills: %w", err)
	}
	defer rows.Close()

	var skills []*models.UserSkill
	for rows.Next() {
		var s models.UserSkill
		var prof sql.NullString
		var created, updated sql.NullTime
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.SkillName, &s.SkillKey, &prof, &created, &updated,
		); err != nil {
			return nil, fmt.Errorf("scan user skill: %w", err)
		}
		s.Proficiency = utils.NullString(prof)
		s.CreatedAt = utils.NullTime(created)
		s.UpdatedAt = utils.NullTime(updated)
		skills = append(skills, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user skills: %w", err)
	}
	return skills, nil
}
