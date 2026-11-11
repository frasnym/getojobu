package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repo "github.com/frasnym/getojobu/apps/backend/internal/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// Job resolves job operations against the repository.
type Job interface {
	Create(ctx context.Context, params models.CreateJobParams) (*models.CreateJobResult, error)
	CreateBulk(ctx context.Context, params models.BulkCreateJobParams) ([]models.BulkCreateJobResult, error)
	GetBySlug(ctx context.Context, slug string) (*models.Job, error)
	IsOwned(ctx context.Context, userID string, jobID int64) (bool, error)
	List(ctx context.Context, limit int, userID string) ([]*models.Job, error)
	SearchByCompany(ctx context.Context, company string, limit int) ([]*models.Job, error)
	UpdateDescription(ctx context.Context, slug, description, updatedByType, updatedByID string) (*models.Job, error)
}

type jobService struct {
	tx       database.Transactor
	repo     repo.Jobs
	userJobs repo.UserJobs
}

// NewJobService creates a job service backed by repositories.
func NewJobService(r *repository.Contract) Job {
	return &jobService{tx: r.Transactor, repo: r.JobsRepository, userJobs: r.UserJobsRepository}
}

// Create inserts a single job listing and creates the ownership row for the
// given user in one transaction. On slug conflict the existing row is kept
// (first-source-wins) and returned with Created=false. If the ownership row
// already exists, AddedToUserList is false.
func (s *jobService) Create(ctx context.Context, params models.CreateJobParams) (*models.CreateJobResult, error) {
	var result *models.CreateJobResult
	operation := func(ctx context.Context) error {
		var err error
		result, err = s.repo.Create(ctx, params)
		if err != nil {
			return err
		}

		added, err := s.userJobs.Create(ctx, params.UserID, result.Job.ID)
		if err != nil {
			return fmt.Errorf("create user_jobs ownership: %w", err)
		}
		result.AddedToUserList = added
		return nil
	}
	if err := s.tx.Transact(ctx, operation); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}
	return result, nil
}

// CreateBulk inserts many job listings in one transaction. Each row uses
// ON CONFLICT (slug) DO NOTHING and gets its own ownership row for the row's
// UserID. All rows run in one transaction (all-or-nothing).
func (s *jobService) CreateBulk(ctx context.Context, params models.BulkCreateJobParams) ([]models.BulkCreateJobResult, error) {
	jobs := params.Jobs
	if len(jobs) == 0 {
		return nil, nil
	}
	if len(jobs) > repo.MaxBulkCreate {
		return nil, fmt.Errorf("bulk create: maximum %d jobs per request, got %d", repo.MaxBulkCreate, len(jobs))
	}

	results := make([]models.BulkCreateJobResult, len(jobs))
	operation := func(ctx context.Context) error {
		createdJobs, err := s.repo.BulkCreate(ctx, params)
		if err != nil {
			return fmt.Errorf("bulk create jobs: %w", err)
		}

		var links []models.UserJobLink
		for i, cj := range createdJobs {
			if cj != nil && cj.Job != nil && jobs[i].UserID != "" {
				links = append(links, models.UserJobLink{
					UserID: jobs[i].UserID,
					JobID:  cj.Job.ID,
				})
			}
		}

		addedMap, err := s.userJobs.BulkCreate(ctx, links)
		if err != nil {
			return fmt.Errorf("bulk create user_jobs: %w", err)
		}

		for i, cj := range createdJobs {
			results[i].Slug = jobs[i].Slug
			if cj != nil {
				results[i].Created = cj.Created
				if cj.Job != nil {
					key := fmt.Sprintf("%s:%d", jobs[i].UserID, cj.Job.ID)
					results[i].AddedToUserList = addedMap[key]
				}
			}
		}
		return nil
	}
	if err := s.tx.Transact(ctx, operation); err != nil {
		return nil, fmt.Errorf("bulk create: %w", err)
	}
	return results, nil
}

// GetBySlug returns the job with the given slug, or ErrNotFound when it does
// not exist.
func (s *jobService) GetBySlug(ctx context.Context, slug string) (*models.Job, error) {
	job, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get job by slug: %w", err)
	}
	return job, nil
}

// List returns the most recent jobs, ordered by created_at descending,
// limited to limit (clamped to [1, 100]). When userID is not empty, only
// jobs owned by that user (via user_jobs) are returned.
func (s *jobService) List(ctx context.Context, limit int, userID string) ([]*models.Job, error) {
	jobs, err := s.repo.List(ctx, limit, userID)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	return jobs, nil
}

// IsOwned reports whether the given user owns the given job.
func (s *jobService) IsOwned(ctx context.Context, userID string, jobID int64) (bool, error) {
	owned, err := s.userJobs.IsOwned(ctx, userID, jobID)
	if err != nil {
		return false, fmt.Errorf("check job ownership: %w", err)
	}
	return owned, nil
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
	// returns constants.ErrNotFound both when the slug is absent and when the guard
	// blocks the update.
	if _, err := s.repo.GetBySlug(ctx, slug); err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get job before description update: %w", err)
	}

	job, err := s.repo.UpdateDescription(ctx, slug, description, updatedByType, updatedByID)
	if err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return nil, constants.ErrDescriptionBlocked
		}
		return nil, fmt.Errorf("update job description: %w", err)
	}
	return job, nil
}
