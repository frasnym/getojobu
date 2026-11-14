package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// Educations provides data access for user_educations.
type Educations interface {
	ListByUser(ctx context.Context, userID string) ([]*models.UserEducation, error)
}

type educationsRepository struct {
	db database.DB
}

// NewEducationsRepository creates an Educations repository backed by the given database.
func NewEducationsRepository(db database.DB) Educations {
	return &educationsRepository{db: db}
}

func (r *educationsRepository) ListByUser(ctx context.Context, userID string) ([]*models.UserEducation, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("list educations by user: invalid user id %w", err)
	}

	query := `
		SELECT id, user_id, institution, degree, field_of_study,
		       start_year, end_year, gpa, is_current, sort_order, created_at, updated_at
		FROM user_educations
		WHERE user_id = $1
		ORDER BY sort_order ASC, COALESCE(end_year, start_year, 0) DESC, created_at DESC
	`

	rows, err := r.db.Query(ctx, query, uid)
	if err != nil {
		return nil, fmt.Errorf("list educations by user: %w", err)
	}
	defer rows.Close()

	var list []*models.UserEducation
	for rows.Next() {
		var edu models.UserEducation
		if err := rows.Scan(
			&edu.ID,
			&edu.UserID,
			&edu.Institution,
			&edu.Degree,
			&edu.FieldOfStudy,
			&edu.StartYear,
			&edu.EndYear,
			&edu.GPA,
			&edu.IsCurrent,
			&edu.SortOrder,
			&edu.CreatedAt,
			&edu.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan education: %w", err)
		}
		list = append(list, &edu)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate educations: %w", err)
	}
	return list, nil
}
