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
	// by created_at descending, limited to limit. Each job carries isOwned=true.
	ListByUser(ctx context.Context, userID string, limit int) ([]*models.Job, error)
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
func (s *userJobsService) ListByUser(ctx context.Context, userID string, limit int) ([]*models.Job, error) {
	jobs, err := s.repo.GetByUser(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list jobs by user: %w", err)
	}
	return jobs, nil
}
