package matching_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

type coverLetterDeps struct {
	evalRepo      *repoMocks.MockMatchingEvaluations
	usersRepo     *repoMocks.MockUsers
	userJobsRepo  *repoMocks.MockUserJobs
	workflowsRepo *repoMocks.MockWorkflows
	service       matching.Matching
}

func newCoverLetterDeps(t *testing.T) *coverLetterDeps {
	t.Helper()
	ctrl := gomock.NewController(t)
	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	usersRepo := repoMocks.NewMockUsers(ctrl)
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)
	workflowsRepo := repoMocks.NewMockWorkflows(ctrl)
	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		UsersRepository:               usersRepo,
		UserJobsRepository:            userJobsRepo,
		WorkflowsRepository:           workflowsRepo,
	})
	return &coverLetterDeps{
		evalRepo:      evalRepo,
		usersRepo:     usersRepo,
		userJobsRepo:  userJobsRepo,
		workflowsRepo: workflowsRepo,
		service:       service,
	}
}

func successfulEvaluation(id, userJobID uuid.UUID) *models.MatchingEvaluation {
	rec := string(constants.RecommendationStrongMatch)
	return &models.MatchingEvaluation{
		ID:                   id,
		UserJobID:            userJobID,
		EvaluationStatus:     string(constants.MatchingStatusSuccess),
		RecommendationLabel:  &rec,
		CandidateProfileData: json.RawMessage(`{"skills":["Go"]}`),
		JobProfileData:       json.RawMessage(`{"title":"Software Engineer"}`),
		EvaluationData:       json.RawMessage(`{"summary":"Strong match"}`),
	}
}

func onboardedUser(id uuid.UUID) *models.User {
	now := time.Now()
	name := "Jane Doe"
	return &models.User{
		ID:                    id.String(),
		FullName:              &name,
		OnboardingCompletedAt: &now,
	}
}

func TestQueueCoverLetter_RejectsOtherUsersEvaluation(t *testing.T) {
	ctx := context.Background()
	d := newCoverLetterDeps(t)

	evalID := uuid.New()
	userJobID := uuid.New()
	ownerID := uuid.New()
	d.evalRepo.EXPECT().GetByID(ctx, evalID).Return(successfulEvaluation(evalID, userJobID), nil)
	d.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: ownerID}, nil)

	caller := &models.Principal{ActorType: "", ID: "not-the-owner"}
	res, err := d.service.QueueCoverLetter(ctx, evalID, matching.QueueCoverLetterRequest{Variant: "short"}, caller)

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrForbidden))
	assert.Nil(t, res)
}

func TestGetEvaluationByUserJobID_IsUserScoped(t *testing.T) {
	ctx := context.Background()
	d := newCoverLetterDeps(t)

	userID := uuid.New()
	otherUserID := uuid.New()
	userJobID := uuid.New()
	evalID := uuid.New()

	t.Run("owner receives computed cover letter file name", func(t *testing.T) {
		eval := successfulEvaluation(evalID, userJobID)
		d.evalRepo.EXPECT().GetLatestByUserJobID(ctx, userID, userJobID).Return(&models.EvaluationWithJob{
			Evaluation: eval,
			Job:        &models.Job{Title: "Software Engineer"},
		}, nil)
		d.usersRepo.EXPECT().FindByID(ctx, userID.String()).Return(onboardedUser(userID), nil)

		res, err := d.service.GetEvaluationByUserJobID(ctx, userID, userJobID)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "Cover Letter_Jane Doe_Software Engineer.pdf", res.Evaluation.CoverLetterFileName)
	})

	t.Run("another user cannot read the evaluation", func(t *testing.T) {
		d.evalRepo.EXPECT().GetLatestByUserJobID(ctx, otherUserID, userJobID).Return(nil, constants.ErrNotFound)

		res, err := d.service.GetEvaluationByUserJobID(ctx, otherUserID, userJobID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrNotFound))
		assert.Nil(t, res)
	})
}

func TestQueueCoverLetter_RequiresSuccessEvaluationAndOnboarding(t *testing.T) {
	ctx := context.Background()

	t.Run("non-success evaluation is rejected", func(t *testing.T) {
		d := newCoverLetterDeps(t)
		evalID := uuid.New()
		userJobID := uuid.New()
		eval := successfulEvaluation(evalID, userJobID)
		eval.EvaluationStatus = string(constants.MatchingStatusPending)
		d.evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)

		res, err := d.service.QueueCoverLetter(ctx, evalID, matching.QueueCoverLetterRequest{Variant: "short"},
			&models.Principal{ActorType: "worker", ID: "worker-1"})

		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrInvalidRequest))
		assert.Nil(t, res)
	})

	t.Run("incomplete onboarding is rejected", func(t *testing.T) {
		d := newCoverLetterDeps(t)
		evalID := uuid.New()
		userJobID := uuid.New()
		userID := uuid.New()
		d.evalRepo.EXPECT().GetByID(ctx, evalID).Return(successfulEvaluation(evalID, userJobID), nil)
		d.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: userID}, nil)
		d.usersRepo.EXPECT().FindByID(ctx, userID.String()).Return(&models.User{ID: userID.String()}, nil)

		res, err := d.service.QueueCoverLetter(ctx, evalID, matching.QueueCoverLetterRequest{Variant: "short"},
			&models.Principal{ActorType: "worker", ID: "worker-1"})

		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrInvalidRequest))
		assert.Contains(t, err.Error(), "onboarding is incomplete")
		assert.Nil(t, res)
	})
}

func TestQueueCoverLetter_ForceRegeneratesAfterSuccess(t *testing.T) {
	ctx := context.Background()
	d := newCoverLetterDeps(t)

	evalID := uuid.New()
	userJobID := uuid.New()
	userID := uuid.New()
	statusSuccess := "success"
	previousContent := json.RawMessage(`{"text":"previous letter","metadata":{"model":"m","prompt_version":"v"}}`)
	previousFailure := "2026-11-16T10:00:00Z — Cover letter generation failed"
	eval := successfulEvaluation(evalID, userJobID)
	eval.CoverLetterStatus = &statusSuccess
	eval.CoverLetterContent = previousContent
	eval.CoverLetterFailureMessage = &previousFailure
	caller := &models.Principal{ActorType: "worker", ID: "worker-1"}

	d.evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
	res, err := d.service.QueueCoverLetter(ctx, evalID, matching.QueueCoverLetterRequest{Variant: "short", Force: false}, caller)
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidRequest))
	assert.Nil(t, res)

	d.evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
	d.userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: userID}, nil)
	d.usersRepo.EXPECT().FindByID(ctx, userID.String()).Return(onboardedUser(userID), nil)
	d.evalRepo.EXPECT().UpdateCoverLetter(ctx, evalID, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ uuid.UUID, params models.UpdateMatchingEvaluationCoverLetterParams) (*models.MatchingEvaluation, error) {
			assert.Equal(t, "queued_retry", params.CoverLetterStatus)
			assert.JSONEq(t, string(previousContent), string(params.CoverLetterContent))
			require.NotNil(t, params.CoverLetterFailureMessage)
			assert.Equal(t, previousFailure, *params.CoverLetterFailureMessage)
			queued := successfulEvaluation(evalID, userJobID)
			queued.CoverLetterStatus = utils.PtrString("queued_retry")
			return queued, nil
		},
	)
	d.workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "cover-letter-gen-"+evalID.String(), models.LimiterJobTypeCoverLetter, gomock.Any()).
		Return("wf-cover-letter-1", nil)

	res, err = d.service.QueueCoverLetter(ctx, evalID, matching.QueueCoverLetterRequest{Variant: "short", Force: true}, caller)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, evalID, *res.ID)
	assert.Equal(t, "queued_retry", *res.CoverLetterStatus)
}

func TestQueueCoverLetter_RejectsUnknownVariant(t *testing.T) {
	ctx := context.Background()
	d := newCoverLetterDeps(t)

	evalID := uuid.New()

	res, err := d.service.QueueCoverLetter(ctx, evalID, matching.QueueCoverLetterRequest{Variant: "medium"},
		&models.Principal{ActorType: "worker", ID: "worker-1"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidRequest))
	assert.Nil(t, res)
}

func TestUpdateEvaluationCoverLetter_KeepsFixedFailureMessage(t *testing.T) {
	ctx := context.Background()
	d := newCoverLetterDeps(t)

	evalID := uuid.New()
	previousContent := json.RawMessage(`{"text":"previous letter","metadata":{"model":"m","prompt_version":"v"}}`)
	existing := &models.MatchingEvaluation{ID: evalID, CoverLetterContent: previousContent}
	d.evalRepo.EXPECT().GetByID(ctx, evalID).Return(existing, nil)

	var captured models.UpdateMatchingEvaluationCoverLetterParams
	d.evalRepo.EXPECT().UpdateCoverLetter(ctx, evalID, gomock.Any()).DoAndReturn(
		func(_ context.Context, id uuid.UUID, params models.UpdateMatchingEvaluationCoverLetterParams) (*models.MatchingEvaluation, error) {
			captured = params
			return &models.MatchingEvaluation{
				ID:                        id,
				CoverLetterStatus:         utils.PtrString("failed"),
				CoverLetterContent:        params.CoverLetterContent,
				CoverLetterFailureMessage: params.CoverLetterFailureMessage,
			}, nil
		},
	)

	rawException := "RuntimeError: could not read CV contents for Jane Doe"
	res, err := d.service.UpdateEvaluationCoverLetter(ctx, evalID, matching.UpdateEvaluationCoverLetterRequest{
		CoverLetterStatus:         "failed",
		CoverLetterFailureMessage: &rawException,
	})

	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, res.CoverLetterFailureMessage)
	assert.JSONEq(t, string(previousContent), string(captured.CoverLetterContent), "failed regeneration must keep the previous letter")
	assert.NotContains(t, *res.CoverLetterFailureMessage, "CV")
	assert.NotContains(t, *res.CoverLetterFailureMessage, "RuntimeError")

	parts := strings.SplitN(*res.CoverLetterFailureMessage, " — ", 2)
	require.Len(t, parts, 2)
	assert.Equal(t, "Cover letter generation failed", parts[1])
	parsed, err := time.Parse(time.RFC3339, parts[0])
	require.NoError(t, err)
	assert.Equal(t, time.UTC, parsed.Location())
}

func TestCoverLetterFileName_Format(t *testing.T) {
	assert.Equal(t, "Cover Letter_Jane Doe_Software Engineer.pdf",
		matching.CoverLetterFileName("Jane Doe", "Software Engineer"))
	assert.Equal(t, "Cover Letter_Jane Doe.pdf",
		matching.CoverLetterFileName("Jane Doe", ""))
	assert.Equal(t, "Cover Letter_Software Engineer.pdf",
		matching.CoverLetterFileName("", "Software Engineer"))
	assert.Equal(t, "Cover Letter.pdf",
		matching.CoverLetterFileName("", ""))
	assert.Equal(t, "Cover Letter_Jane Doe_Senior Engineer.pdf",
		matching.CoverLetterFileName("Jane/ Doe\\", "Senior Engineer"))
}
