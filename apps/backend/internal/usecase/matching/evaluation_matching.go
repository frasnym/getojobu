package matching

import (
	"context"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repo "github.com/frasnym/getojobu/apps/backend/internal/repository"
)

// Matching defines operations for candidate-job compatibility evaluation, report generation, and resume generation.
type Matching interface {
	TriggerCompatibilityBulk(ctx context.Context, items []models.BulkAnalyzeCompatibilityItem, caller *models.Principal) ([]*models.MatchingEvaluationSummary, error)
	UpdateEvaluation(ctx context.Context, id uuid.UUID, req UpdateEvaluationRequest) (*models.MatchingEvaluation, error)
	UpdateEvaluationReport(ctx context.Context, id uuid.UUID, req UpdateEvaluationReportRequest) (*models.MatchingEvaluation, error)
	UpdateEvaluationResume(ctx context.Context, id uuid.UUID, req UpdateEvaluationResumeRequest) (*models.MatchingEvaluation, error)
	GetResume(ctx context.Context, id uuid.UUID, caller *models.Principal) (*models.MatchingEvaluation, error)
	GetReport(ctx context.Context, id uuid.UUID, caller *models.Principal) (*models.MatchingEvaluationReportResponse, error)
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
	}
}
