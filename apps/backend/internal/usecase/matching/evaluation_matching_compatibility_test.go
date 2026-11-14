package matching_test

import (
	"context"
	"encoding/json"
	"errors"
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
		ID:               evalID,
		UserJobID:        userJobID,
		ScoringConfigID:  configID,
		EvaluationStatus: string(constants.MatchingStatusNew),
	}

	t.Run("transition to pending", func(t *testing.T) {
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(existingEval, nil)
		pendingEval := *existingEval
		pendingEval.EvaluationStatus = string(constants.MatchingStatusPending)
		evalRepo.EXPECT().Update(ctx, evalID, models.UpdateMatchingEvaluationParams{
			EvaluationStatus: string(constants.MatchingStatusPending),
		}).Return(&pendingEval, nil)

		res, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			EvaluationStatus: "pending",
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.MatchingStatusPending), res.EvaluationStatus)
	})

	t.Run("transition to pending allowed from new-retry", func(t *testing.T) {
		retryEval := *existingEval
		retryEval.EvaluationStatus = string(constants.MatchingStatusNewRetry)
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(&retryEval, nil)
		pendingEval := retryEval
		pendingEval.EvaluationStatus = string(constants.MatchingStatusPending)
		evalRepo.EXPECT().Update(ctx, evalID, models.UpdateMatchingEvaluationParams{
			EvaluationStatus: string(constants.MatchingStatusPending),
		}).Return(&pendingEval, nil)

		res, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			EvaluationStatus: "pending",
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.MatchingStatusPending), res.EvaluationStatus)
	})

	t.Run("transition to pending rejected when already pending", func(t *testing.T) {
		alreadyPending := *existingEval
		alreadyPending.EvaluationStatus = string(constants.MatchingStatusPending)
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(&alreadyPending, nil)

		_, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			EvaluationStatus: "pending",
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrInvalidTransition))
	})

	t.Run("transition to pending rejected when already success", func(t *testing.T) {
		alreadySuccess := *existingEval
		alreadySuccess.EvaluationStatus = string(constants.MatchingStatusSuccess)
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(&alreadySuccess, nil)

		_, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			EvaluationStatus: "pending",
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrInvalidTransition))
	})

	t.Run("transition to pending rejected when already failed", func(t *testing.T) {
		alreadyFailed := *existingEval
		alreadyFailed.EvaluationStatus = string(constants.MatchingStatusFailed)
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(&alreadyFailed, nil)

		_, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			EvaluationStatus: "pending",
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrInvalidTransition))
	})

	t.Run("transition to failed", func(t *testing.T) {
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(existingEval, nil)
		failMsg := "LLM timeout"
		failedEval := *existingEval
		failedEval.EvaluationStatus = string(constants.MatchingStatusFailed)
		failedEval.FailureMessage = &failMsg
		evalRepo.EXPECT().Update(ctx, evalID, models.UpdateMatchingEvaluationParams{
			EvaluationStatus: string(constants.MatchingStatusFailed),
			FailureMessage:   &failMsg,
		}).Return(&failedEval, nil)

		res, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			EvaluationStatus: "failed",
			FailureMessage:   &failMsg,
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.MatchingStatusFailed), res.EvaluationStatus)
	})

	t.Run("transition to failed appends failure message when already present", func(t *testing.T) {
		oldMsg := "err1"
		priorFailed := *existingEval
		priorFailed.FailureMessage = &oldMsg

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(&priorFailed, nil)
		newMsg := "err2"
		combinedMsg := "err1, err2"
		expectedEval := priorFailed
		expectedEval.EvaluationStatus = string(constants.MatchingStatusFailed)
		expectedEval.FailureMessage = &combinedMsg

		evalRepo.EXPECT().Update(ctx, evalID, models.UpdateMatchingEvaluationParams{
			EvaluationStatus: string(constants.MatchingStatusFailed),
			FailureMessage:   &combinedMsg,
		}).Return(&expectedEval, nil)

		res, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			EvaluationStatus: "failed",
			FailureMessage:   &newMsg,
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.MatchingStatusFailed), res.EvaluationStatus)
		assert.Equal(t, "err1, err2", *res.FailureMessage)
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
		successEval.EvaluationStatus = string(constants.MatchingStatusSuccess)
		successEval.OverallScore = &score
		successEval.EligibilityStatus = &elig
		successEval.RecommendationLabel = &rec

		evalRepo.EXPECT().Update(ctx, evalID, gomock.Any()).Return(&successEval, nil)

		res, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			EvaluationStatus: "success",
			EvaluationData:   evalDataBytes,
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.MatchingStatusSuccess), res.EvaluationStatus)
		assert.Equal(t, 90.0, *res.OverallScore)
	})
}

func TestTriggerCompatibilityBulk(t *testing.T) {
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

	contract := &repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		UserJobsRepository:            userJobsRepo,
		ScoringConfigsRepository:      scoringConfigsRepo,
		JobsRepository:                jobsRepo,
		ExperiencesRepository:         expRepo,
		ProjectsRepository:            projRepo,
		ProjectSkillsRepository:       projSkillsRepo,
		WorkflowsRepository:           workflowsRepo,
	}

	service := matching.NewService(contract)
	ctx := context.Background()

	userID := uuid.New()
	userJobID := uuid.New()
	slug := "test-job"
	job := &models.Job{
		ID:            101,
		Slug:          slug,
		Status:        string(constants.StatusAnalyzed),
		AnalyzeResult: []byte(`{"family":"technology"}`),
	}
	userJob := &models.UserJob{
		ID:     userJobID,
		UserID: userID,
		JobID:  job.ID,
	}
	scoringConfig := &models.ScoringConfig{
		ID:     uuid.New(),
		Family: "technology",
		Categories: []models.CategoryConfig{
			{Name: "technical_skills", Weight: 1.0},
		},
	}

	t.Run("rejects in-flight evaluation even with force=true", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug).Return(job, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil)

		inFlightEval := &models.MatchingEvaluation{
			ID:               uuid.New(),
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusPending),
		}
		evalRepo.EXPECT().GetLatestByUserAndJob(ctx, userID, job.ID).Return(inFlightEval, nil)

		res, err := service.TriggerCompatibilityBulk(ctx, []models.BulkAnalyzeCompatibilityItem{
			{Slug: slug, Force: true},
		}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "already in progress")
	})

	t.Run("rejects success without force", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug).Return(job, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil)

		successEval := &models.MatchingEvaluation{
			ID:               uuid.New(),
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusSuccess),
		}
		evalRepo.EXPECT().GetLatestByUserAndJob(ctx, userID, job.ID).Return(successEval, nil)

		res, err := service.TriggerCompatibilityBulk(ctx, []models.BulkAnalyzeCompatibilityItem{
			{Slug: slug, Force: false},
		}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "already evaluated with status success")
	})

	t.Run("re-evaluates success with force=true creates new record", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug).Return(job, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil)

		successEval := &models.MatchingEvaluation{
			ID:               uuid.New(),
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusSuccess),
		}
		evalRepo.EXPECT().GetLatestByUserAndJob(ctx, userID, job.ID).Return(successEval, nil)

		expRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.Experience{}, nil)
		projRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.Project{}, nil)
		projSkillsRepo.EXPECT().ListSkillsByUser(ctx, userID.String()).Return([]*models.UserSkill{}, nil)

		newEvalID := uuid.New()
		createdEval := &models.MatchingEvaluation{
			ID:               newEvalID,
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusNew),
		}
		evalRepo.EXPECT().Create(ctx, gomock.Any()).Return(createdEval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil)

		res, err := service.TriggerCompatibilityBulk(ctx, []models.BulkAnalyzeCompatibilityItem{
			{Slug: slug, Force: true},
		}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
		assert.Equal(t, newEvalID, *res[0].ID)
		assert.Equal(t, string(constants.MatchingStatusNew), res[0].EvaluationStatus)
	})

	t.Run("rejects failed without force", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug).Return(job, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil)

		failedEval := &models.MatchingEvaluation{
			ID:               uuid.New(),
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusFailed),
		}
		evalRepo.EXPECT().GetLatestByUserAndJob(ctx, userID, job.ID).Return(failedEval, nil)

		res, err := service.TriggerCompatibilityBulk(ctx, []models.BulkAnalyzeCompatibilityItem{
			{Slug: slug, Force: false},
		}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "evaluation failed, force must be true")
	})

	t.Run("re-evaluates failed with force=true updates existing record to new-retry", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug).Return(job, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil)

		failMsg := "original timeout"
		existingFailedID := uuid.New()
		failedEval := &models.MatchingEvaluation{
			ID:               existingFailedID,
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusFailed),
			FailureMessage:   &failMsg,
		}
		evalRepo.EXPECT().GetLatestByUserAndJob(ctx, userID, job.ID).Return(failedEval, nil)

		expRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.Experience{}, nil)
		projRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.Project{}, nil)
		projSkillsRepo.EXPECT().ListSkillsByUser(ctx, userID.String()).Return([]*models.UserSkill{}, nil)

		updatedEval := *failedEval
		updatedEval.EvaluationStatus = string(constants.MatchingStatusNewRetry)
		evalRepo.EXPECT().Update(ctx, existingFailedID, models.UpdateMatchingEvaluationParams{
			EvaluationStatus: string(constants.MatchingStatusNewRetry),
		}).Return(&updatedEval, nil)

		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil)

		res, err := service.TriggerCompatibilityBulk(ctx, []models.BulkAnalyzeCompatibilityItem{
			{Slug: slug, Force: true},
		}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
		assert.Equal(t, existingFailedID, *res[0].ID)
		assert.Equal(t, string(constants.MatchingStatusNewRetry), res[0].EvaluationStatus)
	})

	t.Run("partial batch processing with multiple items", func(t *testing.T) {
		slug1 := "job-1"
		slug2 := "job-2"
		job1 := &models.Job{ID: 101, Slug: slug1, Status: string(constants.StatusAnalyzed), AnalyzeResult: []byte(`{"family":"technology"}`)}
		job2 := &models.Job{ID: 102, Slug: slug2, Status: string(constants.StatusAnalyzed), AnalyzeResult: []byte(`{"family":"technology"}`)}
		userJob1 := &models.UserJob{ID: uuid.New(), UserID: userID, JobID: job1.ID}
		userJob2 := &models.UserJob{ID: uuid.New(), UserID: userID, JobID: job2.ID}

		// Job 1: First time success
		jobsRepo.EXPECT().GetBySlug(ctx, slug1).Return(job1, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job1.ID).Return(true, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job1.ID).Return(userJob1, nil)
		evalRepo.EXPECT().GetLatestByUserAndJob(ctx, userID, job1.ID).Return(nil, constants.ErrNotFound)
		expRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.Experience{}, nil)
		projRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.Project{}, nil)
		projSkillsRepo.EXPECT().ListSkillsByUser(ctx, userID.String()).Return([]*models.UserSkill{}, nil)
		createdEval1 := &models.MatchingEvaluation{ID: uuid.New(), UserJobID: userJob1.ID, EvaluationStatus: string(constants.MatchingStatusNew)}
		evalRepo.EXPECT().Create(ctx, gomock.Any()).Return(createdEval1, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-1", nil)

		// Job 2: Success without force -> Error
		jobsRepo.EXPECT().GetBySlug(ctx, slug2).Return(job2, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job2.ID).Return(true, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job2.ID).Return(userJob2, nil)
		existingSuccess := &models.MatchingEvaluation{ID: uuid.New(), UserJobID: userJob2.ID, EvaluationStatus: string(constants.MatchingStatusSuccess)}
		evalRepo.EXPECT().GetLatestByUserAndJob(ctx, userID, job2.ID).Return(existingSuccess, nil)

		// Job 3: Unknown job -> Error
		jobsRepo.EXPECT().GetBySlug(ctx, "unknown-job").Return(nil, constants.ErrNotFound)

		res, err := service.TriggerCompatibilityBulk(ctx, []models.BulkAnalyzeCompatibilityItem{
			{Slug: slug1, Force: false},
			{Slug: slug2, Force: false},
			{Slug: "unknown-job", Force: false},
		}, &models.Principal{ID: userID.String()})

		require.NoError(t, err)
		require.Len(t, res, 3)

		// Item 1 succeeded
		assert.Equal(t, slug1, res[0].Slug)
		assert.Nil(t, res[0].Error)
		assert.Equal(t, string(constants.MatchingStatusNew), res[0].EvaluationStatus)

		// Item 2 failed with force error
		assert.Equal(t, slug2, res[1].Slug)
		require.NotNil(t, res[1].Error)
		assert.Contains(t, *res[1].Error, "already evaluated with status success")

		// Item 3 failed with not found error
		assert.Equal(t, "unknown-job", res[2].Slug)
		require.NotNil(t, res[2].Error)
		assert.Contains(t, *res[2].Error, "job not found")
	})
}
