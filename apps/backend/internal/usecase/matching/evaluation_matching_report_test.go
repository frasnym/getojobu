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
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)

	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		WorkflowsRepository:           workflowsRepo,
		UserJobsRepository:            userJobsRepo,
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

		queueStatus := string(constants.ReportStatusQueued)
		updatedEval := *eval
		updatedEval.ReportStatus = &queueStatus

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: string(constants.ReportStatusQueued),
		}).Return(&updatedEval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeReport, gomock.Any()).Return("wf-report-1", nil)

		res, err := service.BulkQueueGenerateReport(ctx, []models.BulkGenerateReportItem{
			{ID: evalID, Force: false},
		}, caller)

		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
	})

	t.Run("success marks queued_retry when retrying report", func(t *testing.T) {
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

		queueRetryStatus := string(constants.ReportStatusQueuedRetry)
		updatedEval := *eval
		updatedEval.ReportStatus = &queueRetryStatus

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: string(constants.ReportStatusQueuedRetry),
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

		queueStatus := string(constants.ReportStatusQueued)
		updatedEval := *eval
		updatedEval.ReportStatus = &queueStatus

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: string(constants.ReportStatusQueued),
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

	t.Run("clerk owner of successful evaluation gets queued_retry", func(t *testing.T) {
		userJobID := uuid.New()
		ownerID := uuid.New()
		failedStatus := string(constants.ReportStatusFailed)
		eval := &models.MatchingEvaluation{
			ID:                   evalID,
			UserJobID:            userJobID,
			EvaluationStatus:     string(constants.MatchingStatusSuccess),
			CandidateProfileData: json.RawMessage(`{"skills":["go"]}`),
			JobProfileData:       json.RawMessage(`{"title":"Go Dev"}`),
			EvaluationData:       json.RawMessage(`{"evaluations":[]}`),
			ReportStatus:         &failedStatus,
		}
		queueRetryStatus := string(constants.ReportStatusQueuedRetry)
		updatedEval := *eval
		updatedEval.ReportStatus = &queueRetryStatus

		gomock.InOrder(
			evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil),
			evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil),
			userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
			evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
				ReportStatus: string(constants.ReportStatusQueuedRetry),
			}).Return(&updatedEval, nil),
			workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeReport, gomock.Any()).Return("wf-report-clerk", nil),
		)

		clerkCaller := &models.Principal{ActorType: "", ID: ownerID.String()}
		res, err := service.BulkQueueGenerateReport(ctx, []models.BulkGenerateReportItem{
			{ID: evalID, Force: true},
		}, clerkCaller)

		require.NoError(t, err)
		require.Len(t, res, 1)
		require.Nil(t, res[0].Error)
		require.NotNil(t, res[0].ReportStatus)
		assert.Equal(t, string(constants.ReportStatusQueuedRetry), *res[0].ReportStatus)
	})

	t.Run("clerk caller who does not own the evaluation is rejected", func(t *testing.T) {
		userJobID := uuid.New()
		ownerID := uuid.New()
		eval := &models.MatchingEvaluation{
			ID:               evalID,
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusSuccess),
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil).Times(2)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil)

		clerkCaller := &models.Principal{ActorType: "", ID: uuid.NewString()}
		res, err := service.BulkQueueGenerateReport(ctx, []models.BulkGenerateReportItem{
			{ID: evalID, Force: true},
		}, clerkCaller)

		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "not authorized")
	})

	t.Run("clerk owner cannot retry report when evaluation is not success", func(t *testing.T) {
		userJobID := uuid.New()
		ownerID := uuid.New()
		eval := &models.MatchingEvaluation{
			ID:               evalID,
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusFailed),
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil).Times(2)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil)

		clerkCaller := &models.Principal{ActorType: "", ID: ownerID.String()}
		res, err := service.BulkQueueGenerateReport(ctx, []models.BulkGenerateReportItem{
			{ID: evalID, Force: true},
		}, clerkCaller)

		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "must be success")
	})

	t.Run("api key caller skips the ownership check", func(t *testing.T) {
		eval := &models.MatchingEvaluation{
			ID:                   evalID,
			UserJobID:            uuid.New(),
			EvaluationStatus:     string(constants.MatchingStatusSuccess),
			CandidateProfileData: json.RawMessage(`{}`),
			JobProfileData:       json.RawMessage(`{}`),
			EvaluationData:       json.RawMessage(`{}`),
		}
		queueStatus := string(constants.ReportStatusQueued)
		updatedEval := *eval
		updatedEval.ReportStatus = &queueStatus

		userJobsRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Times(0)
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: string(constants.ReportStatusQueued),
		}).Return(&updatedEval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeReport, gomock.Any()).Return("wf-report-worker", nil)

		res, err := service.BulkQueueGenerateReport(ctx, []models.BulkGenerateReportItem{
			{ID: evalID, Force: false},
		}, caller)

		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
	})
}

func TestRetryReport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	workflowsRepo := repoMocks.NewMockWorkflows(ctrl)
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)

	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		WorkflowsRepository:           workflowsRepo,
		UserJobsRepository:            userJobsRepo,
	})

	ctx := context.Background()
	evalID := uuid.New()
	userJobID := uuid.New()
	ownerID := uuid.New()

	t.Run("clerk non-owner gets a typed forbidden error", func(t *testing.T) {
		eval := &models.MatchingEvaluation{
			ID:               evalID,
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusSuccess),
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil)

		clerkCaller := &models.Principal{ActorType: "", ID: uuid.NewString()}
		summaries, err := service.RetryReport(ctx, evalID, clerkCaller)

		assert.Nil(t, summaries)
		assert.ErrorIs(t, err, constants.ErrForbidden)
	})

	t.Run("clerk caller cannot distinguish a missing evaluation from a forbidden one", func(t *testing.T) {
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(nil, constants.ErrNotFound)

		clerkCaller := &models.Principal{ActorType: "", ID: ownerID.String()}
		summaries, err := service.RetryReport(ctx, evalID, clerkCaller)

		assert.Nil(t, summaries)
		assert.ErrorIs(t, err, constants.ErrForbidden)
	})

	t.Run("clerk owner gets the queued_retry summary", func(t *testing.T) {
		failedStatus := string(constants.ReportStatusFailed)
		eval := &models.MatchingEvaluation{
			ID:                   evalID,
			UserJobID:            userJobID,
			EvaluationStatus:     string(constants.MatchingStatusSuccess),
			CandidateProfileData: json.RawMessage(`{}`),
			JobProfileData:       json.RawMessage(`{}`),
			EvaluationData:       json.RawMessage(`{}`),
			ReportStatus:         &failedStatus,
		}
		queueRetryStatus := string(constants.ReportStatusQueuedRetry)
		updatedEval := *eval
		updatedEval.ReportStatus = &queueRetryStatus

		// up-front authorization check, then the bulk path's own check.
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil).Times(3)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil).Times(2)
		evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: string(constants.ReportStatusQueuedRetry),
		}).Return(&updatedEval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeReport, gomock.Any()).Return("wf-report-retry", nil)

		clerkCaller := &models.Principal{ActorType: "", ID: ownerID.String()}
		summaries, err := service.RetryReport(ctx, evalID, clerkCaller)

		require.NoError(t, err)
		require.Len(t, summaries, 1)
		require.Nil(t, summaries[0].Error)
		require.NotNil(t, summaries[0].ReportStatus)
		assert.Equal(t, string(constants.ReportStatusQueuedRetry), *summaries[0].ReportStatus)
	})

	t.Run("non-success evaluation stays a per-item error for the owner", func(t *testing.T) {
		eval := &models.MatchingEvaluation{
			ID:               evalID,
			UserJobID:        userJobID,
			EvaluationStatus: string(constants.MatchingStatusFailed),
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil).Times(3)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil).Times(2)

		clerkCaller := &models.Principal{ActorType: "", ID: ownerID.String()}
		summaries, err := service.RetryReport(ctx, evalID, clerkCaller)

		require.NoError(t, err)
		require.Len(t, summaries, 1)
		require.NotNil(t, summaries[0].Error)
		assert.Contains(t, *summaries[0].Error, "must be success")
	})

	t.Run("api key caller keeps the bulk path behavior", func(t *testing.T) {
		eval := &models.MatchingEvaluation{
			ID:                   evalID,
			UserJobID:            userJobID,
			EvaluationStatus:     string(constants.MatchingStatusSuccess),
			CandidateProfileData: json.RawMessage(`{}`),
			JobProfileData:       json.RawMessage(`{}`),
			EvaluationData:       json.RawMessage(`{}`),
		}
		queueStatus := string(constants.ReportStatusQueued)
		updatedEval := *eval
		updatedEval.ReportStatus = &queueStatus

		userJobsRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Times(0)
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		evalRepo.EXPECT().UpdateReport(ctx, evalID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: string(constants.ReportStatusQueued),
		}).Return(&updatedEval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeReport, gomock.Any()).Return("wf-report-api-key", nil)

		apiCaller := &models.Principal{ActorType: "worker", ID: "worker-1"}
		summaries, err := service.RetryReport(ctx, evalID, apiCaller)

		require.NoError(t, err)
		require.Len(t, summaries, 1)
		assert.Nil(t, summaries[0].Error)
	})
}
