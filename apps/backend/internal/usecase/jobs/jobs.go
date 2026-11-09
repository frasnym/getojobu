package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// Job resolves job operations against the repository.
type Job interface {
	Create(ctx context.Context, params models.CreateJobParams) (*models.CreateJobResult, error)
	CreateBulk(ctx context.Context, params models.BulkCreateJobParams) ([]models.BulkCreateJobResult, error)
	GetBySlug(ctx context.Context, slug string) (*models.Job, error)
	ListRecent(ctx context.Context, limit int) ([]*models.Job, error)
	SearchByCompany(ctx context.Context, company string, limit int) ([]*models.Job, error)
	UpdateDescription(ctx context.Context, slug, description, updatedByType, updatedByID string) (*models.Job, error)
}

type jobService struct {
	repo repository.Jobs
}

// NewJobService creates a job service backed by a repository.
func NewJobService(repo repository.Jobs) Job {
	return &jobService{repo: repo}
}

// Create inserts a single job listing. On slug conflict the existing row is
// kept (first-source-wins) and returned with Created=false.
func (s *jobService) Create(ctx context.Context, params models.CreateJobParams) (*models.CreateJobResult, error) {
	return s.repo.Create(ctx, params)
}

// CreateBulk inserts many job listings in one request using a pgx.Batch.
// Each row uses ON CONFLICT (slug) DO NOTHING.
func (s *jobService) CreateBulk(ctx context.Context, params models.BulkCreateJobParams) ([]models.BulkCreateJobResult, error) {
	return s.repo.BulkCreate(ctx, params)
}

// GetBySlug returns the job with the given slug, or ErrNotFound when it does
// not exist.
func (s *jobService) GetBySlug(ctx context.Context, slug string) (*models.Job, error) {
	job, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get job by slug: %w", err)
	}
	return job, nil
}

// ListRecent returns the most recent jobs.
func (s *jobService) ListRecent(ctx context.Context, limit int) ([]*models.Job, error) {
	return s.repo.ListRecent(ctx, limit)
}

// SearchByCompany returns jobs whose company matches the given substring
// case-insensitively.
func (s *jobService) SearchByCompany(ctx context.Context, company string, limit int) ([]*models.Job, error) {
	return s.repo.SearchByCompany(ctx, company, limit)
}

// UpdateDescription updates only the description of the job with the given
// slug. It first checks existence (404 when missing) then attempts the
// update, returning ErrDescriptionBlocked (409) when the guard rejects it.
func (s *jobService) UpdateDescription(ctx context.Context, slug, description, updatedByType, updatedByID string) (*models.Job, error) {
	// First check existence so we can distinguish 404 from 409. The repository
	// returns sql.ErrNoRows both when the slug is absent and when the guard
	// blocks the update.
	if _, err := s.repo.GetBySlug(ctx, slug); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get job before description update: %w", err)
	}

	job, err := s.repo.UpdateDescription(ctx, slug, description, updatedByType, updatedByID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrDescriptionBlocked
		}
		return nil, fmt.Errorf("update job description: %w", err)
	}
	return job, nil
}
