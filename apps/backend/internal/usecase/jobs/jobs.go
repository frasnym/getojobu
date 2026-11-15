package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	List(ctx context.Context, limit, offset int, userID string, isCheckOwnership bool) ([]*models.Job, int, error)
	// UpdateStatus validates the status transition and persists the new status.
	// analyzeResult and failureMessage are optional (nil = keep existing value).
	// Returns ErrNotFound when slug is unknown, ErrInvalidTransition when the
	// current → next transition is not permitted.
	UpdateStatus(ctx context.Context, slug, status string, analyzeResult, failureMessage *string, updatedByType, updatedByID string) (*models.Job, error)
	// QueueAnalysisBulk validates each job and queues its JobAnalyzeWorkflow in Temporal.
	QueueAnalysisBulk(ctx context.Context, items []models.QueueAnalysisItem, caller *models.Principal) ([]string, error)
	// StartJobScrape starts JobScrapeWorkflows in Temporal for the normalized
	// search targets derived from user preferences, returning the workflow ids.
	StartJobScrape(ctx context.Context, params models.ScrapeJobsParams) ([]string, error)
}

type jobService struct {
	tx              database.Transactor
	repo            repo.Jobs
	userJobs        repo.UserJobs
	workflows       repo.Workflows
	userPreferences repo.UserPreferences
}

// NewJobService creates a job service backed by repositories.
func NewJobService(r *repository.Contract) Job {
	return &jobService{
		tx:              r.Transactor,
		repo:            r.JobsRepository,
		userJobs:        r.UserJobsRepository,
		workflows:       r.WorkflowsRepository,
		userPreferences: r.UserPreferencesRepository,
	}
}

// Create inserts a single job listing and creates the ownership row for the
// given user in one transaction. On slug conflict the existing row is kept
// (first-source-wins) and returned with Created=false. If the ownership row
// already exists, AddedToUserList is false.
func (s *jobService) Create(ctx context.Context, params models.CreateJobParams) (*models.CreateJobResult, error) {
	if params.Slug == "" {
		params.Slug = models.BuildJobSlug(params.Title, params.Company, params.Location)
	}

	var result *models.CreateJobResult
	operation := func(ctx context.Context) error {
		var err error
		result, err = s.repo.Create(ctx, params)
		if err != nil {
			return err
		}

		for _, uid := range params.UserIDs {
			if uid != "" {
				added, err := s.userJobs.Create(ctx, uid, result.Job.ID)
				if err != nil {
					return fmt.Errorf("create user_jobs ownership: %w", err)
				}
				if added {
					result.AddedToUserList = true
				}
			}
		}
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

	for i := range params.Jobs {
		if params.Jobs[i].Slug == "" {
			params.Jobs[i].Slug = models.BuildJobSlug(params.Jobs[i].Title, params.Jobs[i].Company, params.Jobs[i].Location)
		}
	}
	jobs = params.Jobs

	results := make([]models.BulkCreateJobResult, len(jobs))
	operation := func(ctx context.Context) error {
		createdJobs, err := s.repo.BulkCreate(ctx, params)
		if err != nil {
			return fmt.Errorf("bulk create jobs: %w", err)
		}

		var links []models.UserJobLink
		for i, cj := range createdJobs {
			if cj != nil && cj.Job != nil {
				for _, uid := range jobs[i].UserIDs {
					if uid != "" {
						links = append(links, models.UserJobLink{
							UserID: uid,
							JobID:  cj.Job.ID,
						})
					}
				}
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
					for _, uid := range jobs[i].UserIDs {
						key := fmt.Sprintf("%s:%d", uid, cj.Job.ID)
						if addedMap[key] {
							results[i].AddedToUserList = true
							break
						}
					}
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

// List returns paginated jobs ordered by created_at descending. When isCheckOwnership is true, each
// job is marked with isOwned based on whether userID owns it in user_jobs.
func (s *jobService) List(ctx context.Context, limit, offset int, userID string, isCheckOwnership bool) ([]*models.Job, int, error) {
	jobs, total, err := s.repo.List(ctx, limit, offset, userID, isCheckOwnership)
	if err != nil {
		return nil, 0, fmt.Errorf("list jobs: %w", err)
	}
	return jobs, total, nil
}

// UpdateStatus validates the status transition for the job identified by slug
// and persists the new status together with optional analyzeResult and
// failureMessage fields. Business rule: transition validation happens here, not
// in the handler or the repository. Returns ErrNotFound when slug is unknown
// and ErrInvalidTransition (wrapping the validator's error) when the transition
// is not permitted.
func (s *jobService) UpdateStatus(ctx context.Context, slug, status string, analyzeResult, failureMessage *string, updatedByType, updatedByID string) (*models.Job, error) {
	existing, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get job before status update: %w", err)
	}

	if err := constants.ValidateJobStatusTransition(
		constants.JobStatus(existing.Status),
		constants.JobStatus(status),
	); err != nil {
		return nil, fmt.Errorf("%w: %s", constants.ErrInvalidTransition, err.Error())
	}

	if constants.JobStatus(status) == constants.StatusFailed && failureMessage != nil && existing.FailureMessage != nil && strings.TrimSpace(*existing.FailureMessage) != "" {
		combined := fmt.Sprintf("%s\n%s", *existing.FailureMessage, *failureMessage)
		if len(combined) > 4000 {
			combined = combined[len(combined)-4000:]
		}
		failureMessage = &combined
	}

	job, err := s.repo.UpdateStatus(ctx, slug, status, analyzeResult, failureMessage, updatedByType, updatedByID)
	if err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("update job status: %w", err)
	}
	return job, nil
}
