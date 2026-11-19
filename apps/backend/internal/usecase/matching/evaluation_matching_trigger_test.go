package matching_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

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

type triggerHarness struct {
	service            matching.Matching
	evalRepo           *repoMocks.MockMatchingEvaluations
	userJobsRepo       *repoMocks.MockUserJobs
	scoringConfigsRepo *repoMocks.MockScoringConfigs
	workflowsRepo      *repoMocks.MockWorkflows
}

func newTriggerHarness(t *testing.T) *triggerHarness {
	ctrl := gomock.NewController(t)
	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)
	scoringConfigsRepo := repoMocks.NewMockScoringConfigs(ctrl)
	workflowsRepo := repoMocks.NewMockWorkflows(ctrl)
	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		UserJobsRepository:            userJobsRepo,
		ScoringConfigsRepository:      scoringConfigsRepo,
		WorkflowsRepository:           workflowsRepo,
	})
	return &triggerHarness{
		service:            service,
		evalRepo:           evalRepo,
		userJobsRepo:       userJobsRepo,
		scoringConfigsRepo: scoringConfigsRepo,
		workflowsRepo:      workflowsRepo,
	}
}

func triggerFixture() (ownerID, userJobID, scoringConfigID uuid.UUID) {
	return uuid.New(), uuid.New(), uuid.New()
}

func storedEvaluation(id, userJobID, scoringConfigID uuid.UUID, status string) *models.MatchingEvaluation {
	reportStatus := string(constants.ReportStatusSuccess)
	resumeStatus := string(constants.ResumeStatusSuccess)
	coverStatus := string(constants.CoverLetterStatusSuccess)
	return &models.MatchingEvaluation{
		ID:                   id,
		UserJobID:            userJobID,
		EvaluationStatus:     status,
		CandidateProfileData: []byte(`{"years_experience":5,"skills":["go"]}`),
		JobProfileData:       []byte(`{"family":"technology","title":"Backend Engineer"}`),
		ScoringConfigID:      scoringConfigID,
		EvaluationData:       json.RawMessage(`{"old":true}`),
		FailureMessage:       strPtr("previous timeout"),
		ReportStatus:         &reportStatus,
		ReportData:           json.RawMessage(`{"report":"kept"}`),
		ResumeStatus:         &resumeStatus,
		ResumePDFKey:         strPtr("resume/key.pdf"),
		CoverLetterStatus:    &coverStatus,
		CoverLetterContent:   json.RawMessage(`{"text":"kept"}`),
	}
}

func strPtr(s string) *string { return &s }

func guardedUpdateParams(from, to constants.MatchingEvaluationStatus) models.UpdateMatchingEvaluationParams {
	return models.UpdateMatchingEvaluationParams{
		EvaluationStatus:         string(to),
		ExpectedEvaluationStatus: string(from),
		ClearEvaluationResult:    from != constants.MatchingStatusNew,
	}
}

func triggerScoringConfig(id uuid.UUID) *models.ScoringConfig {
	return &models.ScoringConfig{
		ID:         id,
		Family:     "technology",
		Version:    2,
		IsActive:   true,
		Categories: []models.CategoryConfig{{Name: "technical_skills", Weight: 1.0}},
	}
}

func TestTriggerEvaluations(t *testing.T) {
	ctx := context.Background()

	t.Run("new evaluation starts workflow then flips to queued", func(t *testing.T) {
		h := newTriggerHarness(t)
		ownerID, userJobID, scoringConfigID := triggerFixture()
		id := uuid.New()
		eval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusNew))
		config := triggerScoringConfig(scoringConfigID)

		var capturedInput matching.MatchJobWorkflowInput
		gomock.InOrder(
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil),
			h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
			h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil),
			h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "match-job-"+id.String(), models.LimiterJobTypeMatch, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, _ models.LimiterJobType, input any) (string, error) {
					capturedInput, _ = input.(matching.MatchJobWorkflowInput)
					return "wf-id", nil
				}),
			h.evalRepo.EXPECT().Update(ctx, id, guardedUpdateParams(constants.MatchingStatusNew, constants.MatchingStatusQueued)).Return(nil, nil),
		)

		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id}}, &models.Principal{ID: ownerID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
		assert.Equal(t, id, *res[0].ID)
		assert.Equal(t, string(constants.MatchingStatusQueued), res[0].EvaluationStatus)

		assert.Equal(t, id.String(), capturedInput.EvaluationID)
		assert.Equal(t, "Backend Engineer", capturedInput.JobProfile["title"])
		assert.Equal(t, []interface{}{"go"}, capturedInput.CandidateProfile["skills"])
		assert.Equal(t, "technology", capturedInput.EvaluationConfig["family"])
		for _, key := range []string{"id", "version", "is_active", "created_at", "updated_at"} {
			_, exists := capturedInput.EvaluationConfig[key]
			assert.False(t, exists, "evaluation_config should not contain %q", key)
		}
	})

	t.Run("failed evaluation uses one atomic clear-and-flip after starting workflow", func(t *testing.T) {
		h := newTriggerHarness(t)
		ownerID, userJobID, scoringConfigID := triggerFixture()
		id := uuid.New()
		eval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusFailed))
		config := triggerScoringConfig(scoringConfigID)

		var capturedInput matching.MatchJobWorkflowInput
		// The guarded update clears results and flips status in one call. The
		// recorded order proves the stored profile and scoring config were
		// decoded into the workflow input before the row is mutated.
		gomock.InOrder(
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil),
			h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
			h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil),
			h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "match-job-"+id.String(), models.LimiterJobTypeMatch, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, _ models.LimiterJobType, input any) (string, error) {
					capturedInput, _ = input.(matching.MatchJobWorkflowInput)
					return "wf-id", nil
				}),
			h.evalRepo.EXPECT().Update(ctx, id, guardedUpdateParams(constants.MatchingStatusFailed, constants.MatchingStatusQueuedRetry)).Return(nil, nil),
		)

		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id}}, &models.Principal{ID: ownerID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
		assert.Equal(t, string(constants.MatchingStatusQueuedRetry), res[0].EvaluationStatus)
		assert.Equal(t, "Backend Engineer", capturedInput.JobProfile["title"])
		assert.Equal(t, []interface{}{"go"}, capturedInput.CandidateProfile["skills"])
		assert.Equal(t, "technology", capturedInput.EvaluationConfig["family"])
	})

	t.Run("second failure keeps profile bytes and feeds a non-empty workflow input", func(t *testing.T) {
		h := newTriggerHarness(t)
		ownerID, userJobID, scoringConfigID := triggerFixture()
		id := uuid.New()
		// After one failed retry the result columns have been cleared, but the
		// profile bytes are inputs and must still be present for this retry.
		eval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusFailed))
		eval.EvaluationData = nil
		eval.FailureMessage = nil
		eval.OverallScore = nil
		eval.EligibilityStatus = nil
		eval.RecommendationLabel = nil
		config := triggerScoringConfig(scoringConfigID)

		var capturedInput matching.MatchJobWorkflowInput
		gomock.InOrder(
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil),
			h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
			h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil),
			h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "match-job-"+id.String(), models.LimiterJobTypeMatch, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, _ models.LimiterJobType, input any) (string, error) {
					capturedInput, _ = input.(matching.MatchJobWorkflowInput)
					return "wf-id", nil
				}),
			h.evalRepo.EXPECT().Update(ctx, id, guardedUpdateParams(constants.MatchingStatusFailed, constants.MatchingStatusQueuedRetry)).Return(nil, nil),
		)

		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id}}, &models.Principal{ID: ownerID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
		assert.NotEmpty(t, capturedInput.CandidateProfile, "candidate profile bytes must survive a failed retry")
		assert.NotEmpty(t, capturedInput.JobProfile, "job profile bytes must survive a failed retry")
		assert.Equal(t, "Backend Engineer", capturedInput.JobProfile["title"])
	})

	t.Run("worker pending race is reported as an in-flight success", func(t *testing.T) {
		h := newTriggerHarness(t)
		ownerID, userJobID, scoringConfigID := triggerFixture()
		id := uuid.New()
		eval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusNew))
		pendingEval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusPending))
		config := triggerScoringConfig(scoringConfigID)

		gomock.InOrder(
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil),
			h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
			h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil),
			h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "match-job-"+id.String(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil),
			h.evalRepo.EXPECT().Update(ctx, id, guardedUpdateParams(constants.MatchingStatusNew, constants.MatchingStatusQueued)).Return(nil, constants.ErrNotFound),
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(pendingEval, nil),
		)

		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id}}, &models.Principal{ID: ownerID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error, "an already-running workflow is not a failure")
		assert.Equal(t, string(constants.MatchingStatusPending), res[0].EvaluationStatus)
	})

	t.Run("guard mismatch with no in-flight status is a per-item error", func(t *testing.T) {
		h := newTriggerHarness(t)
		ownerID, userJobID, scoringConfigID := triggerFixture()
		id := uuid.New()
		eval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusNew))
		failedEval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusFailed))
		config := triggerScoringConfig(scoringConfigID)

		gomock.InOrder(
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil),
			h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
			h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil),
			h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "match-job-"+id.String(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil),
			h.evalRepo.EXPECT().Update(ctx, id, guardedUpdateParams(constants.MatchingStatusNew, constants.MatchingStatusQueued)).Return(nil, constants.ErrNotFound),
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(failedEval, nil),
		)

		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id}}, &models.Principal{ID: ownerID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "status changed before it could be queued")
	})

	t.Run("other update errors stay per-item errors", func(t *testing.T) {
		h := newTriggerHarness(t)
		ownerID, userJobID, scoringConfigID := triggerFixture()
		id := uuid.New()
		eval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusNew))
		config := triggerScoringConfig(scoringConfigID)

		gomock.InOrder(
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil),
			h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
			h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil),
			h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "match-job-"+id.String(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil),
			h.evalRepo.EXPECT().Update(ctx, id, guardedUpdateParams(constants.MatchingStatusNew, constants.MatchingStatusQueued)).Return(nil, errors.New("db down")),
		)

		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id}}, &models.Principal{ID: ownerID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "update evaluation status: db down")
	})

	t.Run("in-progress statuses are rejected without force and start nothing", func(t *testing.T) {
		for _, status := range []string{
			string(constants.MatchingStatusQueued),
			string(constants.MatchingStatusQueuedRetry),
			string(constants.MatchingStatusPending),
		} {
			t.Run(status, func(t *testing.T) {
				h := newTriggerHarness(t)
				ownerID, userJobID, scoringConfigID := triggerFixture()
				id := uuid.New()
				eval := storedEvaluation(id, userJobID, scoringConfigID, status)

				h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil)
				h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil)
				h.workflowsRepo.EXPECT().StartLimiterWorkItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

				res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id}}, &models.Principal{ID: ownerID.String()})
				require.NoError(t, err)
				require.Len(t, res, 1)
				require.NotNil(t, res[0].Error)
				assert.Contains(t, *res[0].Error, status)
				assert.Contains(t, *res[0].Error, "force is required")
			})
		}
	})

	t.Run("success is terminal and is rejected even with force", func(t *testing.T) {
		for _, force := range []bool{false, true} {
			t.Run(fmt.Sprintf("force=%t", force), func(t *testing.T) {
				h := newTriggerHarness(t)
				ownerID, userJobID, scoringConfigID := triggerFixture()
				id := uuid.New()
				eval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusSuccess))

				h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil)
				h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil)
				h.workflowsRepo.EXPECT().StartLimiterWorkItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

				res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id, Force: force}}, &models.Principal{ID: ownerID.String()})
				require.NoError(t, err)
				require.Len(t, res, 1)
				require.NotNil(t, res[0].Error)
				assert.Contains(t, *res[0].Error, string(constants.MatchingStatusSuccess))
			})
		}
	})

	t.Run("forced in-progress rows requeue with a nonce-suffixed workflow ID and clear results", func(t *testing.T) {
		for _, status := range []constants.MatchingEvaluationStatus{
			constants.MatchingStatusQueued,
			constants.MatchingStatusQueuedRetry,
			constants.MatchingStatusPending,
		} {
			t.Run(string(status), func(t *testing.T) {
				h := newTriggerHarness(t)
				ownerID, userJobID, scoringConfigID := triggerFixture()
				id := uuid.New()
				eval := storedEvaluation(id, userJobID, scoringConfigID, string(status))
				eval.UpdatedAt = time.Unix(1_700_000_000, 123456789)
				config := triggerScoringConfig(scoringConfigID)

				gomock.InOrder(
					h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil),
					h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
					h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil),
					h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx,
						fmt.Sprintf("match-job-%s-%d", id.String(), eval.UpdatedAt.UnixNano()),
						models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil),
					h.evalRepo.EXPECT().Update(ctx, id, guardedUpdateParams(status, constants.MatchingStatusQueuedRetry)).Return(nil, nil),
				)

				res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id, Force: true}}, &models.Principal{ID: ownerID.String()})
				require.NoError(t, err)
				require.Len(t, res, 1)
				assert.Nil(t, res[0].Error)
				assert.Equal(t, string(constants.MatchingStatusQueuedRetry), res[0].EvaluationStatus)
			})
		}
	})

	t.Run("non-force new evaluation keeps the deterministic workflow ID", func(t *testing.T) {
		h := newTriggerHarness(t)
		ownerID, userJobID, scoringConfigID := triggerFixture()
		id := uuid.New()
		eval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusNew))
		eval.UpdatedAt = time.Unix(1_700_000_000, 987654321)
		config := triggerScoringConfig(scoringConfigID)

		gomock.InOrder(
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil),
			h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
			h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil),
			h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "match-job-"+id.String(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil),
			h.evalRepo.EXPECT().Update(ctx, id, guardedUpdateParams(constants.MatchingStatusNew, constants.MatchingStatusQueued)).Return(nil, nil),
		)

		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id, Force: false}}, &models.Principal{ID: ownerID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
	})

	t.Run("second trigger on the same row starts one workflow", func(t *testing.T) {
		h := newTriggerHarness(t)
		ownerID, userJobID, scoringConfigID := triggerFixture()
		id := uuid.New()
		newEval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusNew))
		queuedEval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusQueued))
		config := triggerScoringConfig(scoringConfigID)

		gomock.InOrder(
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(newEval, nil),
			h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
			h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil),
			h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "match-job-"+id.String(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil),
			h.evalRepo.EXPECT().Update(ctx, id, guardedUpdateParams(constants.MatchingStatusNew, constants.MatchingStatusQueued)).Return(nil, nil),
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(queuedEval, nil),
			h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
		)

		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id}, {ID: id}}, &models.Principal{ID: ownerID.String()})
		require.NoError(t, err)
		require.Len(t, res, 2)
		assert.Nil(t, res[0].Error)
		require.NotNil(t, res[1].Error)
		assert.Contains(t, *res[1].Error, string(constants.MatchingStatusQueued))
	})

	t.Run("workflow start failure leaves the row unflipped", func(t *testing.T) {
		h := newTriggerHarness(t)
		ownerID, userJobID, scoringConfigID := triggerFixture()
		id := uuid.New()
		eval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusNew))
		config := triggerScoringConfig(scoringConfigID)

		h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil)
		h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil)
		h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil)
		h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "match-job-"+id.String(), models.LimiterJobTypeMatch, gomock.Any()).Return("", errors.New("temporal down"))
		h.evalRepo.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id}}, &models.Principal{ID: ownerID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "start match job workflow")
	})

	t.Run("clerk caller cannot trigger another user's evaluation", func(t *testing.T) {
		h := newTriggerHarness(t)
		ownerID, userJobID, scoringConfigID := triggerFixture()
		intruderID := uuid.New()
		id := uuid.New()
		eval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusNew))

		h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil)
		h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil)

		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id}}, &models.Principal{ID: intruderID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "another user")
	})

	t.Run("api key caller triggers the evaluation without a user check", func(t *testing.T) {
		h := newTriggerHarness(t)
		_, userJobID, scoringConfigID := triggerFixture()
		id := uuid.New()
		eval := storedEvaluation(id, userJobID, scoringConfigID, string(constants.MatchingStatusNew))
		config := triggerScoringConfig(scoringConfigID)

		gomock.InOrder(
			h.evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil),
			h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil),
			h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "match-job-"+id.String(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil),
			h.evalRepo.EXPECT().Update(ctx, id, guardedUpdateParams(constants.MatchingStatusNew, constants.MatchingStatusQueued)).Return(nil, nil),
		)
		h.userJobsRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Times(0)

		caller := &models.Principal{ActorType: "api_key", ActorID: "key-1"}
		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: id}}, caller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
		assert.Equal(t, string(constants.MatchingStatusQueued), res[0].EvaluationStatus)
	})

	t.Run("one bad item does not stop the rest of the batch", func(t *testing.T) {
		h := newTriggerHarness(t)
		ownerID, userJobID, scoringConfigID := triggerFixture()
		badID := uuid.New()
		goodID := uuid.New()
		badEval := storedEvaluation(badID, userJobID, scoringConfigID, string(constants.MatchingStatusSuccess))
		goodEval := storedEvaluation(goodID, userJobID, scoringConfigID, string(constants.MatchingStatusNew))
		config := triggerScoringConfig(scoringConfigID)

		gomock.InOrder(
			h.evalRepo.EXPECT().GetByID(ctx, badID).Return(badEval, nil),
			h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
			h.evalRepo.EXPECT().GetByID(ctx, goodID).Return(goodEval, nil),
			h.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil),
			h.scoringConfigsRepo.EXPECT().GetByID(ctx, scoringConfigID).Return(config, nil),
			h.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "match-job-"+goodID.String(), models.LimiterJobTypeMatch, gomock.Any()).Return("wf-id", nil),
			h.evalRepo.EXPECT().Update(ctx, goodID, guardedUpdateParams(constants.MatchingStatusNew, constants.MatchingStatusQueued)).Return(nil, nil),
		)

		res, err := h.service.TriggerEvaluations(ctx, []models.TriggerEvaluationItem{{ID: badID}, {ID: goodID}}, &models.Principal{ID: ownerID.String()})
		require.NoError(t, err)
		require.Len(t, res, 2)
		require.NotNil(t, res[0].Error)
		assert.Nil(t, res[1].Error)
		assert.Equal(t, goodID, *res[1].ID)
	})
}
