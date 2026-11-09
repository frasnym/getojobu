package projects

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// Project resolves project operations against the repository.
type Project interface {
	Create(ctx context.Context, params models.CreateProjectParams) (*models.Project, error)
	GetByID(ctx context.Context, params models.GetProjectParams) (*models.Project, error)
	ListByUser(ctx context.Context, userID string) ([]*models.Project, error)
	Update(ctx context.Context, params models.UpdateProjectParams) (*models.Project, error)
}

// projectService implements Project backed by a repository.
type projectService struct {
	repo repository.Projects
}

// NewProjectService creates a project service backed by a repository.
func NewProjectService(repo repository.Projects) Project {
	return &projectService{repo: repo}
}

// Create inserts a new project after validating input.
func (s *projectService) Create(ctx context.Context, params models.CreateProjectParams) (*models.Project, error) {
	if params.StartedAt != nil && params.EndedAt != nil && *params.EndedAt < *params.StartedAt {
		return nil, fmt.Errorf("create project: ended_at must be >= started_at")
	}

	if params.IsCurrent && params.EndedAt != nil {
		return nil, fmt.Errorf("create project: a current project must not have an ended_at")
	}

	return s.repo.Create(ctx, params)
}

// GetByID returns the project with the given id owned by userID.
func (s *projectService) GetByID(ctx context.Context, params models.GetProjectParams) (*models.Project, error) {
	p, err := s.repo.GetByID(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	return p, nil
}

// ListByUser returns all projects for the given user.
func (s *projectService) ListByUser(ctx context.Context, userID string) ([]*models.Project, error) {
	return s.repo.ListByUser(ctx, userID)
}

// Update modifies an existing project after validating input.
func (s *projectService) Update(ctx context.Context, params models.UpdateProjectParams) (*models.Project, error) {
	if params.StartedAt != nil && params.EndedAt != nil && *params.EndedAt < *params.StartedAt {
		return nil, fmt.Errorf("update project: ended_at must be >= started_at")
	}

	if params.IsCurrent && params.EndedAt != nil {
		return nil, fmt.Errorf("update project: a current project must not have an ended_at")
	}

	p, err := s.repo.Update(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("update project: %w", err)
	}
	return p, nil
}
