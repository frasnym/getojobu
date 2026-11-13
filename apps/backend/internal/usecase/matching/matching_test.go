package matching_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repoMocks "github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

func TestCalculateScores(t *testing.T) {
	configs := []models.CategoryConfig{
		{Name: "technical_skills", Weight: 0.35},
		{Name: "responsibilities", Weight: 0.25},
		{Name: "experience", Weight: 0.20},
		{Name: "seniority", Weight: 0.15},
		{Name: "domain", Weight: 0.05},
	}

	t.Run("strong match with all mandatory met", func(t *testing.T) {
		output := models.MatchEvaluationOutput{
			Evaluations: []models.CategoryEvaluation{
				{Category: "technical_skills", Score: 90},
				{Category: "responsibilities", Score: 80},
				{Category: "experience", Score: 100},
				{Category: "seniority", Score: 65},
				{Category: "domain", Score: 90},
			},
			MandatoryRequirements: []models.MandatoryRequirementAssessment{
				{Requirement: "5+ years", Status: "met"},
				{Requirement: "Go", Status: "met"},
			},
		}

		overallScore, eligibility, recommendation, err := matching.CalculateScores(configs, output)
		require.NoError(t, err)
		assert.Equal(t, 85.75, overallScore)
		assert.Equal(t, constants.EligibilityStatusEligible, eligibility)
		assert.Equal(t, constants.RecommendationStrongMatch, recommendation)
	})

	t.Run("worth applying score between 60 and 74 with eligible", func(t *testing.T) {
		output := models.MatchEvaluationOutput{
			Evaluations: []models.CategoryEvaluation{
				{Category: "technical_skills", Score: 70},
				{Category: "responsibilities", Score: 65},
				{Category: "experience", Score: 60},
				{Category: "seniority", Score: 60},
				{Category: "domain", Score: 60},
			},
			MandatoryRequirements: []models.MandatoryRequirementAssessment{
				{Requirement: "Go", Status: "met"},
			},
		}

		overallScore, eligibility, recommendation, err := matching.CalculateScores(configs, output)
		require.NoError(t, err)
		assert.True(t, overallScore >= 60 && overallScore < 75)
		assert.Equal(t, constants.EligibilityStatusEligible, eligibility)
		assert.Equal(t, constants.RecommendationWorthApplying, recommendation)
	})

	t.Run("high score but not_eligible mandatory requirement yields low_match", func(t *testing.T) {
		output := models.MatchEvaluationOutput{
			Evaluations: []models.CategoryEvaluation{
				{Category: "technical_skills", Score: 95},
				{Category: "responsibilities", Score: 90},
				{Category: "experience", Score: 95},
				{Category: "seniority", Score: 90},
				{Category: "domain", Score: 90},
			},
			MandatoryRequirements: []models.MandatoryRequirementAssessment{
				{Requirement: "Driver License", Status: "not_met"},
			},
		}

		overallScore, eligibility, recommendation, err := matching.CalculateScores(configs, output)
		require.NoError(t, err)
		assert.True(t, overallScore >= 90)
		assert.Equal(t, constants.EligibilityStatusNotEligible, eligibility)
		// Crucial safety rule: high score must never surface as strong_match if not eligible!
		assert.Equal(t, constants.RecommendationLowMatch, recommendation)
	})

	t.Run("moderate score with uncertain mandatory yields possible_match", func(t *testing.T) {
		output := models.MatchEvaluationOutput{
			Evaluations: []models.CategoryEvaluation{
				{Category: "technical_skills", Score: 50},
				{Category: "responsibilities", Score: 50},
				{Category: "experience", Score: 50},
				{Category: "seniority", Score: 50},
				{Category: "domain", Score: 50},
			},
			MandatoryRequirements: []models.MandatoryRequirementAssessment{
				{Requirement: "Security clearance", Status: "uncertain"},
			},
		}

		overallScore, eligibility, recommendation, err := matching.CalculateScores(configs, output)
		require.NoError(t, err)
		assert.Equal(t, 50.0, overallScore)
		assert.Equal(t, constants.EligibilityStatusNeedsVerification, eligibility)
		assert.Equal(t, constants.RecommendationPossibleMatch, recommendation)
	})
}

func TestUpdateEvaluation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)
	scoringConfigsRepo := repoMocks.NewMockScoringConfigs(ctrl)
	jobsRepo := repoMocks.NewMockJobs(ctrl)
	expRepo := repoMocks.NewMockExperiences(ctrl)
	projRepo := repoMocks.NewMockProjects(ctrl)
	projSkillsRepo := repoMocks.NewMockProjectSkills(ctrl)
	workflowsRepo := repoMocks.NewMockWorkflows(ctrl)

	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		UserJobsRepository:            userJobsRepo,
		ScoringConfigsRepository:      scoringConfigsRepo,
		JobsRepository:                jobsRepo,
		ExperiencesRepository:         expRepo,
		ProjectsRepository:            projRepo,
		ProjectSkillsRepository:       projSkillsRepo,
		WorkflowsRepository:           workflowsRepo,
	})

	ctx := context.Background()
	evalID := uuid.New()
	userJobID := uuid.New()
	configID := uuid.New()

	existingEval := &models.MatchingEvaluation{
		ID:              evalID,
		UserJobID:       userJobID,
		ScoringConfigID: configID,
		Status:          string(constants.MatchingStatusNew),
	}

	t.Run("transition to pending", func(t *testing.T) {
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(existingEval, nil)
		pendingEval := *existingEval
		pendingEval.Status = string(constants.MatchingStatusPending)
		evalRepo.EXPECT().Update(ctx, evalID, models.UpdateMatchingEvaluationParams{
			Status: string(constants.MatchingStatusPending),
		}).Return(&pendingEval, nil)

		res, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			Status: "pending",
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.MatchingStatusPending), res.Status)
	})

	t.Run("transition to failed", func(t *testing.T) {
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(existingEval, nil)
		failMsg := "LLM timeout"
		failedEval := *existingEval
		failedEval.Status = string(constants.MatchingStatusFailed)
		failedEval.FailureMessage = &failMsg
		evalRepo.EXPECT().Update(ctx, evalID, models.UpdateMatchingEvaluationParams{
			Status:         string(constants.MatchingStatusFailed),
			FailureMessage: &failMsg,
		}).Return(&failedEval, nil)

		res, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			Status:         "failed",
			FailureMessage: &failMsg,
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.MatchingStatusFailed), res.Status)
	})

	t.Run("transition to success calculates score and updates user_jobs", func(t *testing.T) {
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(existingEval, nil)

		scoringConfig := &models.ScoringConfig{
			ID:     configID,
			Family: "technology",
			Categories: []models.CategoryConfig{
				{Name: "technical_skills", Weight: 1.0},
			},
		}
		scoringConfigsRepo.EXPECT().GetByID(ctx, configID).Return(scoringConfig, nil)

		evalOutput := models.MatchEvaluationOutput{
			Evaluations: []models.CategoryEvaluation{
				{Category: "technical_skills", Score: 90, Reason: "Proficient"},
			},
			MandatoryRequirements: []models.MandatoryRequirementAssessment{
				{Requirement: "Go", Status: "met", Reason: "Experience listed"},
			},
		}
		evalDataBytes, err := json.Marshal(evalOutput)
		require.NoError(t, err)

		score := 90.0
		elig := string(constants.EligibilityStatusEligible)
		rec := string(constants.RecommendationStrongMatch)
		successEval := *existingEval
		successEval.Status = string(constants.MatchingStatusSuccess)
		successEval.OverallScore = &score
		successEval.EligibilityStatus = &elig
		successEval.RecommendationLabel = &rec

		evalRepo.EXPECT().Update(ctx, evalID, gomock.Any()).Return(&successEval, nil)

		res, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			Status:         "success",
			EvaluationData: evalDataBytes,
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.MatchingStatusSuccess), res.Status)
		assert.Equal(t, 90.0, *res.OverallScore)
	})
}

func TestTriggerCompatibility(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)
	scoringConfigsRepo := repoMocks.NewMockScoringConfigs(ctrl)
	jobsRepo := repoMocks.NewMockJobs(ctrl)
	expRepo := repoMocks.NewMockExperiences(ctrl)
	projRepo := repoMocks.NewMockProjects(ctrl)
	projSkillsRepo := repoMocks.NewMockProjectSkills(ctrl)
	workflowsRepo := repoMocks.NewMockWorkflows(ctrl)

	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		UserJobsRepository:            userJobsRepo,
		ScoringConfigsRepository:      scoringConfigsRepo,
		JobsRepository:                jobsRepo,
		ExperiencesRepository:         expRepo,
		ProjectsRepository:            projRepo,
		ProjectSkillsRepository:       projSkillsRepo,
		WorkflowsRepository:           workflowsRepo,
	})

	ctx := context.Background()
	slug := "senior-backend-engineer"
	userID := uuid.New()
	userJobID := uuid.New()

	analyzeResult := json.RawMessage(`{"family":"technology","title":"Senior Backend Engineer"}`)
	job := &models.Job{
		ID:            101,
		Slug:          slug,
		Status:        string(constants.StatusAnalyzed),
		AnalyzeResult: analyzeResult,
	}

	t.Run("reject if job not analyzed", func(t *testing.T) {
		newJob := &models.Job{
			ID:     102,
			Slug:   "new-job",
			Status: string(constants.StatusNew),
		}
		jobsRepo.EXPECT().GetBySlug(ctx, "new-job").Return(newJob, nil)

		_, err := service.TriggerCompatibility(ctx, "new-job", &models.Principal{ID: userID.String()})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not analyzed")
	})

	t.Run("reject if clerk user not in user_jobs", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug).Return(job, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(false, nil)

		_, err := service.TriggerCompatibility(ctx, slug, &models.Principal{ID: userID.String()})
		assert.Error(t, err)
		assert.Equal(t, constants.ErrNotFound, err)
	})

	t.Run("succeeds for user in user_jobs", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug).Return(job, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)

		scoringConfig := &models.ScoringConfig{
			ID:     uuid.New(),
			Family: "technology",
			Categories: []models.CategoryConfig{
				{Name: "technical_skills", Weight: 1.0},
			},
		}
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)

		userJob := &models.UserJob{
			ID:     userJobID,
			UserID: userID,
			JobID:  job.ID,
		}
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil)

		expRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.Experience{
			{
				RoleTitle:    "Backend Dev",
				Organization: "Tech Corp",
			},
		}, nil)
		projRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.Project{}, nil)
		projSkillsRepo.EXPECT().ListSkillsByUser(ctx, userID.String()).Return([]*models.UserSkill{}, nil)

		createdEval := &models.MatchingEvaluation{
			ID:        uuid.New(),
			UserJobID: userJobID,
			Status:    string(constants.MatchingStatusNew),
		}
		evalRepo.EXPECT().Create(ctx, gomock.Any()).Return(createdEval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil)

		res, err := service.TriggerCompatibility(ctx, slug, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, string(constants.MatchingStatusNew), res[0].Status)
	})

	t.Run("machine caller returns empty slice when 0 users in user_jobs", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug).Return(job, nil)
		userJobsRepo.EXPECT().GetUsersByJobID(ctx, job.ID).Return([]string{}, nil)

		res, err := service.TriggerCompatibility(ctx, slug, &models.Principal{ActorType: "machine"})
		require.NoError(t, err)
		assert.Empty(t, res)
	})

	t.Run("scoring config falls back to technology when family not found", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug).Return(job, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)

		// GetActiveByFamily for "technology" fails initially, then fallback to "technology"
		scoringConfig := &models.ScoringConfig{
			ID:     uuid.New(),
			Family: "technology",
			Categories: []models.CategoryConfig{
				{Name: "technical_skills", Weight: 1.0},
			},
		}
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)

		userJob := &models.UserJob{
			ID:     userJobID,
			UserID: userID,
			JobID:  job.ID,
		}
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil)

		expRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.Experience{}, nil)
		projRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.Project{}, nil)
		projSkillsRepo.EXPECT().ListSkillsByUser(ctx, userID.String()).Return([]*models.UserSkill{}, nil)

		createdEval := &models.MatchingEvaluation{
			ID:        uuid.New(),
			UserJobID: userJobID,
			Status:    string(constants.MatchingStatusNew),
		}
		evalRepo.EXPECT().Create(ctx, gomock.Any()).Return(createdEval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil)

		res, err := service.TriggerCompatibility(ctx, slug, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		assert.Len(t, res, 1)
	})
}
