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

func lowMatchEvalData(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal(models.MatchEvaluationOutput{
		Evaluations: []models.CategoryEvaluation{
			{Category: "technical_skills", Score: 90, Reason: "Proficient"},
		},
		MandatoryRequirements: []models.MandatoryRequirementAssessment{
			{Requirement: "Driver License", Status: "not_met", Reason: "Missing"},
		},
	})
	require.NoError(t, err)
	return data
}

func lowMatchSetup(
	t *testing.T,
	ctx context.Context,
	evalRepo *repoMocks.MockMatchingEvaluations,
	scoringConfigsRepo *repoMocks.MockScoringConfigs,
	evalID, configID uuid.UUID,
	existingEval *models.MatchingEvaluation,
) {
	t.Helper()
	evalRepo.EXPECT().GetByID(ctx, evalID).Return(existingEval, nil)
	scoringConfigsRepo.EXPECT().GetByID(ctx, configID).Return(&models.ScoringConfig{
		ID:     configID,
		Family: "technology",
		Categories: []models.CategoryConfig{
			{Name: "technical_skills", Weight: 1.0},
		},
	}, nil)

	score := 90.0
	eligibility := string(constants.EligibilityStatusNotEligible)
	recommendation := string(constants.RecommendationLowMatch)
	updated := *existingEval
	updated.EvaluationStatus = string(constants.MatchingStatusSuccess)
	updated.OverallScore = &score
	updated.EligibilityStatus = &eligibility
	updated.RecommendationLabel = &recommendation
	evalRepo.EXPECT().Update(ctx, evalID, gomock.Any()).Return(&updated, nil)
}

func TestUpdateEvaluation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)
	userPrefsRepo := repoMocks.NewMockUserPreferences(ctrl)
	scoringConfigsRepo := repoMocks.NewMockScoringConfigs(ctrl)
	jobsRepo := repoMocks.NewMockJobs(ctrl)
	expRepo := repoMocks.NewMockExperiences(ctrl)
	projRepo := repoMocks.NewMockProjects(ctrl)
	projSkillsRepo := repoMocks.NewMockProjectSkills(ctrl)
	workflowsRepo := repoMocks.NewMockWorkflows(ctrl)

	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		UserJobsRepository:            userJobsRepo,
		UserPreferencesRepository:     userPrefsRepo,
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
		EvaluationStatus: string(constants.MatchingStatusQueued),
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

	t.Run("transition to pending allowed from queued_retry", func(t *testing.T) {
		retryEval := *existingEval
		retryEval.EvaluationStatus = string(constants.MatchingStatusQueuedRetry)
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

	t.Run("low match hides the user job when auto hide is enabled", func(t *testing.T) {
		lowMatchSetup(t, ctx, evalRepo, scoringConfigsRepo, evalID, configID, existingEval)

		userID := uuid.New()
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID:     userJobID,
			UserID: userID,
		}, nil)
		userPrefsRepo.EXPECT().IsAutoHideLowMatchEnabled(ctx, userID.String()).Return(true, nil)
		userJobsRepo.EXPECT().SetBulkHideStatus(ctx, userID.String(), []string{userJobID.String()}, true).Return(1, nil)

		res, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			EvaluationStatus: "success",
			EvaluationData:   lowMatchEvalData(t),
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.RecommendationLowMatch), *res.RecommendationLabel)
	})

	t.Run("low match leaves the user job visible when auto hide is disabled", func(t *testing.T) {
		lowMatchSetup(t, ctx, evalRepo, scoringConfigsRepo, evalID, configID, existingEval)

		userID := uuid.New()
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID:     userJobID,
			UserID: userID,
		}, nil)
		userPrefsRepo.EXPECT().IsAutoHideLowMatchEnabled(ctx, userID.String()).Return(false, nil)

		res, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			EvaluationStatus: "success",
			EvaluationData:   lowMatchEvalData(t),
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.RecommendationLowMatch), *res.RecommendationLabel)
	})

	t.Run("low match fails the update when the hide cannot be applied", func(t *testing.T) {
		lowMatchSetup(t, ctx, evalRepo, scoringConfigsRepo, evalID, configID, existingEval)

		userID := uuid.New()
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID:     userJobID,
			UserID: userID,
		}, nil)
		userPrefsRepo.EXPECT().IsAutoHideLowMatchEnabled(ctx, userID.String()).Return(true, nil)
		userJobsRepo.EXPECT().SetBulkHideStatus(ctx, userID.String(), []string{userJobID.String()}, true).Return(0, errors.New("hide DB down"))

		_, err := service.UpdateEvaluation(ctx, evalID, matching.UpdateEvaluationRequest{
			EvaluationStatus: "success",
			EvaluationData:   lowMatchEvalData(t),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "auto hide low match evaluation")
	})
}
