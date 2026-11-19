package user_jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repo "github.com/frasnym/getojobu/apps/backend/internal/repository"
	ucJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
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

	// CreateManual inserts a private job owned by the calling user, source=manual,
	// with a namespaced slug and the usual user_jobs ownership link.
	CreateManual(ctx context.Context, userID string, params models.CreateManualJobParams) (*models.CreateJobResult, error)

	// ScrapeURL creates a scrape_pending skeleton job for a user-supplied URL
	// and dispatches UserJobScrapeWorkflow through the limiter.
	ScrapeURL(ctx context.Context, userID string, params models.ScrapeURLParams) (*models.CreateJobResult, error)
}

// userJobsService implements UserJobs backed by the user_jobs repository.
type userJobsService struct {
	repo      repo.UserJobs
	jobsUC    ucJobs.Job
	workflows repo.Workflows
}

// NewUserJobsService creates a user-jobs service backed by the repository.
func NewUserJobsService(r *repository.Contract, jobsUC ucJobs.Job) UserJobs {
	return &userJobsService{
		repo:      r.UserJobsRepository,
		jobsUC:    jobsUC,
		workflows: r.WorkflowsRepository,
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

// CreateManual inserts a private job owned by the calling user and links it to
// them through the jobs usecase. The slug is prefixed with the user's first
// eight UUID characters so manual jobs cannot collide with scraped listings.
func (s *userJobsService) CreateManual(ctx context.Context, userID string, params models.CreateManualJobParams) (*models.CreateJobResult, error) {
	if strings.TrimSpace(params.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	if strings.TrimSpace(params.Company) == "" {
		return nil, fmt.Errorf("company is required")
	}
	if strings.TrimSpace(params.Location) == "" {
		return nil, fmt.Errorf("location is required")
	}

	ownerID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	createParams := models.CreateJobParams{
		Slug:           models.BuildManualJobSlug(userID, params.Title, params.Company, params.Location),
		Source:         "manual",
		OwnerID:        &ownerID,
		SourceURL:      params.SourceURL,
		ApplyURL:       params.ApplyURL,
		Title:          params.Title,
		Company:        params.Company,
		Location:       params.Location,
		Description:    params.Description,
		PostedDate:     params.PostedDate,
		IsRemote:       params.IsRemote,
		VisaRelocation: params.VisaRelocation,
		UpdatedByType:  "user",
		UpdatedByID:    userID,
		UserIDs:        []string{userID},
	}

	result, err := s.jobsUC.Create(ctx, createParams)
	if err != nil {
		return nil, fmt.Errorf("create manual job: %w", err)
	}
	return result, nil
}

// ScrapeURL creates a skeleton job at scrape_pending and queues the scrape
// workflow. If the workflow cannot be started, the job is marked
// scrape_failed with the dispatch error so the user can retry.
func (s *userJobsService) ScrapeURL(ctx context.Context, userID string, params models.ScrapeURLParams) (*models.CreateJobResult, error) {
	if strings.TrimSpace(params.SourceURL) == "" {
		return nil, fmt.Errorf("source_url is required")
	}

	ownerID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	slugPrefix := userID
	if len(userID) >= 8 {
		slugPrefix = userID[:8]
	}
	slug := fmt.Sprintf("m-%s-scrape-%d", slugPrefix, time.Now().UnixNano())

	createParams := models.CreateJobParams{
		Slug:          slug,
		Source:        "user_scrape",
		Status:        string(constants.StatusScrapePending),
		OwnerID:       &ownerID,
		SourceURL:     &params.SourceURL,
		Title:         "Pending scrape",
		Company:       "unknown",
		Location:      "unknown",
		UpdatedByType: "user",
		UpdatedByID:   userID,
		UserIDs:       []string{userID},
	}

	result, err := s.jobsUC.Create(ctx, createParams)
	if err != nil {
		return nil, fmt.Errorf("create scrape job: %w", err)
	}

	_, err = s.workflows.StartUserJobScrape(ctx, models.UserJobScrapeWorkflowInput{
		JobID:     result.Job.ID,
		Slug:      result.Job.Slug,
		SourceURL: params.SourceURL,
		UserID:    userID,
		OwnerID:   userID,
	})
	if err != nil {
		// Move the skeleton to scrape_failed so the UI offers a retry instead of
		// leaving the job spinning at scrape_pending forever.
		message := fmt.Sprintf("scrape dispatch failed: %v", err)
		if _, updateErr := s.jobsUC.UpdateStatus(ctx, models.UpdateJobParams{
			Slug:           result.Job.Slug,
			Status:         string(constants.StatusScrapeFailed),
			FailureMessage: &message,
			UpdatedByType:  "user",
			UpdatedByID:    userID,
		}); updateErr != nil {
			return nil, fmt.Errorf("mark scrape failed after dispatch error: %w", updateErr)
		}
		result.Job.Status = string(constants.StatusScrapeFailed)
		result.Job.FailureMessage = &message
	}
	return result, nil
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
