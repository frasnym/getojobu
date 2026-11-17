package matching

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repo "github.com/frasnym/getojobu/apps/backend/internal/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// Matching defines operations for candidate-job compatibility evaluation, report generation, and resume generation.
type Matching interface {
	TriggerCompatibilityBulk(ctx context.Context, items []models.BulkAnalyzeCompatibilityItem, caller *models.Principal) ([]*models.MatchingEvaluationSummary, error)
	UpdateEvaluation(ctx context.Context, id uuid.UUID, req UpdateEvaluationRequest) (*models.MatchingEvaluation, error)
	UpdateEvaluationReport(ctx context.Context, id uuid.UUID, req UpdateEvaluationReportRequest) (*models.MatchingEvaluation, error)
	UpdateEvaluationResume(ctx context.Context, id uuid.UUID, req UpdateEvaluationResumeRequest) (*models.MatchingEvaluation, error)
	GetResume(ctx context.Context, id uuid.UUID, caller *models.Principal) (*models.MatchingEvaluation, error)
	DownloadResume(ctx context.Context, id uuid.UUID, caller *models.Principal) (*models.DownloadResumeResponse, error)
	GetEvaluationByUserJobID(ctx context.Context, userID uuid.UUID, userJobID uuid.UUID) (*models.EvaluationWithJob, error)
	QueueCoverLetter(ctx context.Context, id uuid.UUID, req QueueCoverLetterRequest, caller *models.Principal) (*models.MatchingEvaluationSummary, error)
	UpdateEvaluationCoverLetter(ctx context.Context, id uuid.UUID, req UpdateEvaluationCoverLetterRequest) (*models.MatchingEvaluation, error)
	BulkQueueGenerateReport(ctx context.Context, items []models.BulkGenerateReportItem, caller *models.Principal) ([]*models.MatchingEvaluationSummary, error)
	BulkQueueGenerateResume(ctx context.Context, items []models.BulkGenerateResumeItem, caller *models.Principal) ([]*models.MatchingEvaluationSummary, error)
}

type matchingService struct {
	evalRepo           repo.MatchingEvaluations
	usersRepo          repo.Users
	userJobsRepo       repo.UserJobs
	scoringConfigsRepo repo.ScoringConfigs
	jobsRepo           repo.Jobs
	expRepo            repo.Experiences
	projRepo           repo.Projects
	projSkillsRepo     repo.ProjectSkills
	workflowsRepo      repo.Workflows
	eduRepo            repo.Educations
	storageRepo        repo.Storage
}

// NewService constructs a new Matching usecase service.
func NewService(r *repository.Contract) Matching {
	return &matchingService{
		evalRepo:           r.MatchingEvaluationsRepository,
		usersRepo:          r.UsersRepository,
		userJobsRepo:       r.UserJobsRepository,
		scoringConfigsRepo: r.ScoringConfigsRepository,
		jobsRepo:           r.JobsRepository,
		expRepo:            r.ExperiencesRepository,
		projRepo:           r.ProjectsRepository,
		projSkillsRepo:     r.ProjectSkillsRepository,
		workflowsRepo:      r.WorkflowsRepository,
		eduRepo:            r.EducationsRepository,
		storageRepo:        r.StorageRepository,
	}
}

// GetEvaluationByUserJobID returns the latest matching evaluation for a
// user-job link with its job listing, verifying ownership via user_id.
func (s *matchingService) GetEvaluationByUserJobID(ctx context.Context, userID uuid.UUID, userJobID uuid.UUID) (*models.EvaluationWithJob, error) {
	// The repository scopes the lookup by user_id, so another user's user-job
	// ID resolves to ErrNotFound rather than leaking the evaluation.
	result, err := s.evalRepo.GetLatestByUserJobID(ctx, userID, userJobID)
	if err != nil {
		return nil, err
	}
	if result == nil || result.Evaluation == nil {
		return nil, constants.ErrNotFound
	}

	user, err := s.usersRepo.FindByID(ctx, userID.String())
	if err != nil {
		return nil, fmt.Errorf("get cover letter user: %w", err)
	}
	name := ""
	if user != nil {
		name = utils.PtrStringOrEmpty(user.FullName)
	}
	jobTitle := ""
	if result.Job != nil {
		jobTitle = result.Job.Title
	}
	result.Evaluation.CoverLetterFileName = CoverLetterFileName(name, jobTitle)
	return result, nil
}
