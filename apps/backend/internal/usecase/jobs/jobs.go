package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

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
	GetBySlug(ctx context.Context, slug string, userID string) (*models.JobWithOwnership, error)
	List(ctx context.Context, limit, offset int, userID string) ([]*models.JobWithOwnership, int, error)
	// UpdateStatus validates the status transition and persists the new status.
	// analyzeResult and failureMessage are optional (nil = keep existing value).
	// Returns ErrNotFound when slug is unknown, ErrInvalidTransition when the
	// current → next transition is not permitted.
	UpdateStatus(ctx context.Context, params models.UpdateJobParams) (*models.Job, error)
	// QueueAnalysisBulk validates each job and queues its JobAnalyzeWorkflow in Temporal.
	// Per-item failures are returned in the result instead of aborting the batch.
	QueueAnalysisBulk(ctx context.Context, items []models.QueueAnalysisItem, caller *models.Principal) (models.QueueAnalysisBulkResult, error)
	// StartJobScrape starts JobScrapeWorkflows in Temporal for the normalized
	// search targets derived from user preferences, returning the workflow ids.
	StartJobScrape(ctx context.Context, params models.ScrapeJobsParams) ([]string, error)
	// CreateManual inserts a private job owned by the calling user.
	CreateManual(ctx context.Context, userID string, params models.CreateManualJobParams) (*models.CreateJobResult, error)
	// ScrapeURL creates a scrape_pending skeleton job and dispatches the URL scrape workflow.
	ScrapeURL(ctx context.Context, userID string, params models.ScrapeURLParams) (*models.CreateJobResult, error)
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

// CreateManual inserts a private job owned by the calling user and links it to
// them. The slug is prefixed with the user's first eight UUID characters so
// manual jobs cannot collide with scraped listings or with other users.
func (s *jobService) CreateManual(ctx context.Context, userID string, params models.CreateManualJobParams) (*models.CreateJobResult, error) {
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

	return s.Create(ctx, models.CreateJobParams{
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
	})
}

// ScrapeURL creates a skeleton job at scrape_pending and queues the scrape
// workflow. If the workflow cannot be started, the job is marked scrape_failed
// with the dispatch error so the user can retry.
func (s *jobService) ScrapeURL(ctx context.Context, userID string, params models.ScrapeURLParams) (*models.CreateJobResult, error) {
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

	result, err := s.Create(ctx, models.CreateJobParams{
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
	})
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
		message := fmt.Sprintf("scrape dispatch failed: %v", err)
		if _, updateErr := s.UpdateStatus(ctx, models.UpdateJobParams{
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
// not exist. When userID is non-empty, ownership is resolved for that user.
func (s *jobService) GetBySlug(ctx context.Context, slug string, userID string) (*models.JobWithOwnership, error) {
	job, err := s.repo.GetBySlug(ctx, slug, userID)
	if err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get job by slug: %w", err)
	}
	return job, nil
}

// List returns paginated jobs ordered by created_at descending. When userID is
// non-empty, ownership is resolved via user_jobs for that user.
func (s *jobService) List(ctx context.Context, limit, offset int, userID string) ([]*models.JobWithOwnership, int, error) {
	jobs, total, err := s.repo.List(ctx, limit, offset, userID)
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
func (s *jobService) UpdateStatus(ctx context.Context, params models.UpdateJobParams) (*models.Job, error) {
	existingRes, err := s.repo.GetBySlug(ctx, params.Slug, "")
	if err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get job before status update: %w", err)
	}
	existing := existingRes.Job

	if err := constants.ValidateJobStatusTransition(
		constants.JobStatus(existing.Status),
		constants.JobStatus(params.Status),
	); err != nil {
		return nil, fmt.Errorf("%w: %s", constants.ErrInvalidTransition, err.Error())
	}

	if constants.JobStatus(params.Status) == constants.StatusFailed && params.FailureMessage != nil && existing.FailureMessage != nil && strings.TrimSpace(*existing.FailureMessage) != "" {
		combined := fmt.Sprintf("%s\n%s", *existing.FailureMessage, *params.FailureMessage)
		if len(combined) > 4000 {
			combined = combined[len(combined)-4000:]
		}
		params.FailureMessage = &combined
	}

	// Extended fields are meaningful only for the scrape callback. For every
	// other transition they are cleared so existing rows are left untouched.
	if constants.JobStatus(existing.Status) != constants.StatusScrapePending {
		params.NewSlug = nil
		params.Title = nil
		params.Company = nil
		params.Location = nil
		params.Description = nil
		params.SourceURL = nil
		params.ApplyURL = nil
		params.PostedDate = nil
		params.IsRemote = nil
		params.VisaRelocation = nil
	}

	job, err := s.repo.UpdateStatus(ctx, params)
	if err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("update job status: %w", err)
	}
	return job, nil
}
