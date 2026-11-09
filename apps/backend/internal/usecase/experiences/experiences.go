package experiences

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// Experience resolves experience operations against the repository.
type Experience interface {
	Create(ctx context.Context, params models.CreateExperienceParams) (*models.Experience, error)
	GetByID(ctx context.Context, params models.GetExperienceParams) (*models.Experience, error)
	ListByUser(ctx context.Context, userID string) ([]*models.Experience, error)
	Update(ctx context.Context, params models.UpdateExperienceParams) (*models.Experience, error)
}

// experienceService implements Experience backed by a repository.
type experienceService struct {
	repo repository.Experiences
}

// NewExperienceService creates an experience service backed by a repository.
func NewExperienceService(repo repository.Experiences) Experience {
	return &experienceService{repo: repo}
}

// Create inserts a new experience after validating input.
func (s *experienceService) Create(ctx context.Context, params models.CreateExperienceParams) (*models.Experience, error) {
	if !params.ExperienceType.IsValid() {
		return nil, fmt.Errorf("create experience: invalid experience_type %q", params.ExperienceType)
	}

	if params.StartedAt != nil && params.EndedAt != nil && *params.EndedAt < *params.StartedAt {
		return nil, fmt.Errorf("create experience: ended_at must be >= started_at")
	}

	if params.IsCurrent && params.EndedAt != nil {
		return nil, fmt.Errorf("create experience: a current experience must not have an ended_at")
	}

	return s.repo.Create(ctx, params)
}

// GetByID returns the experience with the given id owned by userID.
func (s *experienceService) GetByID(ctx context.Context, params models.GetExperienceParams) (*models.Experience, error) {
	e, err := s.repo.GetByID(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get experience: %w", err)
	}
	return e, nil
}

// ListByUser returns all experiences for the given user.
func (s *experienceService) ListByUser(ctx context.Context, userID string) ([]*models.Experience, error) {
	return s.repo.ListByUser(ctx, userID)
}

// Update modifies an existing experience after validating input.
func (s *experienceService) Update(ctx context.Context, params models.UpdateExperienceParams) (*models.Experience, error) {
	if !params.ExperienceType.IsValid() {
		return nil, fmt.Errorf("update experience: invalid experience_type %q", params.ExperienceType)
	}

	if params.StartedAt != nil && params.EndedAt != nil && *params.EndedAt < *params.StartedAt {
		return nil, fmt.Errorf("update experience: ended_at must be >= started_at")
	}

	if params.IsCurrent && params.EndedAt != nil {
		return nil, fmt.Errorf("update experience: a current experience must not have an ended_at")
	}

	e, err := s.repo.Update(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("update experience: %w", err)
	}
	return e, nil
}
