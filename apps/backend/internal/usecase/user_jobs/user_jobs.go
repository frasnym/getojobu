package user_jobs

import (
	"context"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repo "github.com/frasnym/getojobu/apps/backend/internal/repository"
)

// UserJobs resolves user-job ownership operations against the repository.
// user_jobs is a separate table from jobs: it records which user owns which
// shared job listing.
type UserJobs interface {
	// ListByUser returns jobs owned by the given user (via user_jobs), ordered
	// by is_hidden ascending (visible jobs first) and then created_at descending,
	// paginated by limit and offset.
	ListByUser(ctx context.Context, userID string, filter models.UserJobFilter, limit, offset int) ([]*models.UserJobWithJob, int, error)

	// Add links the given job to the given user in user_jobs. Returns created=true
	// when a new ownership row was inserted, created=false when the pair already
	// exists (idempotent via ON CONFLICT DO NOTHING).
	Add(ctx context.Context, userID string, jobID int64) (created bool, err error)

	// SetBulkHideStatus sets is_hidden on multiple user↔job links owned by the
	// user in one transaction. Returns the number of rows actually updated.
	SetBulkHideStatus(ctx context.Context, userID string, userJobIDs []string, isHidden bool) (int, error)
}

// userJobsService implements UserJobs backed by the user_jobs repository.
type userJobsService struct {
	repo repo.UserJobs
}

// NewUserJobsService creates a user-jobs service backed by the repository.
func NewUserJobsService(r *repository.Contract) UserJobs {
	return &userJobsService{
		repo: r.UserJobsRepository,
	}
}

// ListByUser returns jobs owned by the given user.
func (s *userJobsService) ListByUser(ctx context.Context, userID string, filter models.UserJobFilter, limit, offset int) ([]*models.UserJobWithJob, int, error) {
	jobs, total, err := s.repo.GetByUser(ctx, userID, filter, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list jobs by user: %w", err)
	}
	return jobs, total, nil
}

// Add links the given job to the given user in user_jobs. Returns created=true
// when a new ownership row was inserted, created=false when the pair already
// exists (idempotent).
func (s *userJobsService) Add(ctx context.Context, userID string, jobID int64) (bool, error) {
	created, err := s.repo.Create(ctx, userID, jobID)
	if err != nil {
		return false, fmt.Errorf("add user job: %w", err)
	}
	return created, nil
}

// SetBulkHideStatus sets is_hidden on multiple user↔job links owned by the
// user in one transaction. Returns the number of rows actually updated.
func (s *userJobsService) SetBulkHideStatus(ctx context.Context, userID string, userJobIDs []string, isHidden bool) (int, error) {
	updated, err := s.repo.SetBulkHideStatus(ctx, userID, userJobIDs, isHidden)
	if err != nil {
		return 0, fmt.Errorf("set bulk hide status: %w", err)
	}
	return updated, nil
}
