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

func TestGetReport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)

	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		UserJobsRepository:            userJobsRepo,
	})

	ctx := context.Background()
	evalID := uuid.New()
	userJobID := uuid.New()
	ownerUserID := uuid.New()

	t.Run("unauthorized when caller is nil", func(t *testing.T) {
		res, err := service.GetReport(ctx, evalID, nil)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, constants.ErrUnauthorized)
	})

	t.Run("forbidden when Clerk user does not match UserID", func(t *testing.T) {
		otherUserID := uuid.New()
		eval := &models.MatchingEvaluation{
			ID:               evalID,
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusSuccess),
		}
		userJob := &models.UserJob{
			ID:     userJobID,
			UserID: ownerUserID,
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(userJob, nil)

		res, err := service.GetReport(ctx, evalID, &models.Principal{ID: otherUserID.String()})
		assert.Nil(t, res)
		assert.ErrorIs(t, err, constants.ErrForbidden)
	})

	t.Run("success when Clerk user matches UserID", func(t *testing.T) {
		repStatus := string(constants.ReportStatusSuccess)
		score := 90.0
		eval := &models.MatchingEvaluation{
			ID:               evalID,
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusSuccess),
			ReportStatus:     &repStatus,
			OverallScore:     &score,
			ReportData:       json.RawMessage(`{"job_title":"Go Engineer"}`),
		}
		userJob := &models.UserJob{
			ID:     userJobID,
			UserID: ownerUserID,
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(userJob, nil)

		res, err := service.GetReport(ctx, evalID, &models.Principal{ID: ownerUserID.String()})
		require.NoError(t, err)
		assert.Equal(t, evalID, res.ID)
		assert.Equal(t, repStatus, *res.ReportStatus)
		assert.Equal(t, score, *res.OverallScore)
	})

	t.Run("success when machine API key caller bypasses user ownership", func(t *testing.T) {
		repStatus := string(constants.ReportStatusSuccess)
		eval := &models.MatchingEvaluation{
			ID:               evalID,
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusSuccess),
			ReportStatus:     &repStatus,
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)

		res, err := service.GetReport(ctx, evalID, &models.Principal{ID: "machine-1", ActorType: "api_key"})
		require.NoError(t, err)
		assert.Equal(t, evalID, res.ID)
	})
}

func TestUpdateEvaluationReport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)

	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
	})

	ctx := context.Background()
	evalID := uuid.New()

	t.Run("success updating report", func(t *testing.T) {
		repStatus := string(constants.ReportStatusSuccess)
		eval := &models.MatchingEvaluation{
			ID:           evalID,
			ReportStatus: &repStatus,
			ReportData:   json.RawMessage(`{"job_title":"Go Engineer"}`),
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: string(constants.ReportStatusSuccess),
			ReportData:   json.RawMessage(`{"job_title":"Go Engineer"}`),
		}).Return(eval, nil)

		res, err := service.UpdateEvaluationReport(ctx, evalID, matching.UpdateEvaluationReportRequest{
			ReportStatus: string(constants.ReportStatusSuccess),
			ReportData:   json.RawMessage(`{"job_title":"Go Engineer"}`),
		})
		require.NoError(t, err)
		assert.Equal(t, evalID, res.ID)
		assert.Equal(t, repStatus, *res.ReportStatus)
	})

	t.Run("returns not found when eval does not exist", func(t *testing.T) {
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(nil, constants.ErrNotFound)

		res, err := service.UpdateEvaluationReport(ctx, evalID, matching.UpdateEvaluationReportRequest{
			ReportStatus: string(constants.ReportStatusSuccess),
		})
		assert.Nil(t, res)
		assert.ErrorIs(t, err, constants.ErrNotFound)
	})

	t.Run("preserves and combines failure message when updating report", func(t *testing.T) {
		repStatus := string(constants.ReportStatusFailed)
		existingMsg := "previous timeout"
		eval := &models.MatchingEvaluation{
			ID:                   evalID,
			ReportStatus:         &repStatus,
			ReportFailureMessage: &existingMsg,
		}

		newMsg := "rate limit exceeded"
		expectedCombined := "previous timeout, rate limit exceeded"

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus:         string(constants.ReportStatusFailed),
			ReportFailureMessage: &expectedCombined,
		}).Return(eval, nil)

		res, err := service.UpdateEvaluationReport(ctx, evalID, matching.UpdateEvaluationReportRequest{
			ReportStatus:         string(constants.ReportStatusFailed),
			ReportFailureMessage: &newMsg,
		})
		require.NoError(t, err)
		assert.Equal(t, evalID, res.ID)
	})
}

func TestBulkQueueGenerateReport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	workflowsRepo := repoMocks.NewMockWorkflows(ctrl)

	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		WorkflowsRepository:           workflowsRepo,
	})

	ctx := context.Background()
	evalID := uuid.New()
	caller := &models.Principal{ID: "worker-1", ActorType: "api_key"}

	t.Run("unauthorized when caller is nil", func(t *testing.T) {
		res, err := service.BulkQueueGenerateReport(ctx, []models.BulkGenerateReportItem{{ID: evalID}}, nil)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, constants.ErrUnauthorized)
	})

	t.Run("rejects when already pending without force", func(t *testing.T) {
		pendingStatus := string(constants.ReportStatusPending)
		eval := &models.MatchingEvaluation{
			ID:               evalID,
			EvaluationStatus: string(constants.MatchingStatusSuccess),
			ReportStatus:     &pendingStatus,
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)

		res, err := service.BulkQueueGenerateReport(ctx, []models.BulkGenerateReportItem{
			{ID: evalID, Force: false},
		}, caller)

		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "already in progress")
	})

	t.Run("success marks pending and starts limiter workflow", func(t *testing.T) {
		eval := &models.MatchingEvaluation{
			ID:                   evalID,
			UserJobID:            uuid.New(),
			EvaluationStatus:     string(constants.MatchingStatusSuccess),
			CandidateProfileData: json.RawMessage(`{"skills":["go"]}`),
			JobProfileData:       json.RawMessage(`{"title":"Go Dev"}`),
			EvaluationData:       json.RawMessage(`{"evaluations":[]}`),
		}

		pendingStatus := string(constants.ReportStatusPending)
		updatedEval := *eval
		updatedEval.ReportStatus = &pendingStatus

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: string(constants.ReportStatusPending),
		}).Return(&updatedEval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeReport, gomock.Any()).Return("wf-report-1", nil)

		res, err := service.BulkQueueGenerateReport(ctx, []models.BulkGenerateReportItem{
			{ID: evalID, Force: false},
		}, caller)

		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
	})

	t.Run("success marks pending-retry when retrying report", func(t *testing.T) {
		failedStatus := string(constants.ReportStatusFailed)
		eval := &models.MatchingEvaluation{
			ID:                   evalID,
			UserJobID:            uuid.New(),
			EvaluationStatus:     string(constants.MatchingStatusSuccess),
			CandidateProfileData: json.RawMessage(`{"skills":["go"]}`),
			JobProfileData:       json.RawMessage(`{"title":"Go Dev"}`),
			EvaluationData:       json.RawMessage(`{"evaluations":[]}`),
			ReportStatus:         &failedStatus,
		}

		pendingRetryStatus := string(constants.ReportStatusPendingRetry)
		updatedEval := *eval
		updatedEval.ReportStatus = &pendingRetryStatus

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: string(constants.ReportStatusPendingRetry),
		}).Return(&updatedEval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeReport, gomock.Any()).Return("wf-report-2", nil)

		res, err := service.BulkQueueGenerateReport(ctx, []models.BulkGenerateReportItem{
			{ID: evalID, Force: true},
		}, caller)

		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
	})

	t.Run("marks failed in DB when limiter fails to start", func(t *testing.T) {
		eval := &models.MatchingEvaluation{
			ID:                   evalID,
			UserJobID:            uuid.New(),
			EvaluationStatus:     string(constants.MatchingStatusSuccess),
			CandidateProfileData: json.RawMessage(`{}`),
			JobProfileData:       json.RawMessage(`{}`),
			EvaluationData:       json.RawMessage(`{}`),
		}

		pendingStatus := string(constants.ReportStatusPending)
		updatedEval := *eval
		updatedEval.ReportStatus = &pendingStatus

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: string(constants.ReportStatusPending),
		}).Return(&updatedEval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeReport, gomock.Any()).Return("", errors.New("temporal connection error"))

		// Expect compensating update setting report_status = failed
		evalRepo.EXPECT().UpdateReport(ctx, evalID, gomock.Any()).DoAndReturn(
			func(ctx context.Context, id uuid.UUID, params models.UpdateMatchingEvaluationReportParams) (*models.MatchingEvaluation, error) {
				assert.Equal(t, string(constants.ReportStatusFailed), params.ReportStatus)
				require.NotNil(t, params.ReportFailureMessage)
				assert.Contains(t, *params.ReportFailureMessage, "temporal connection error")
				return eval, nil
			},
		)

		res, err := service.BulkQueueGenerateReport(ctx, []models.BulkGenerateReportItem{
			{ID: evalID, Force: false},
		}, caller)

		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "temporal connection error")
	})
}
