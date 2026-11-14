package matching_test

import (
	"context"
	"encoding/json"
	"errors"
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

func TestBulkQueueGenerateResume(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	usersRepo := repoMocks.NewMockUsers(ctrl)
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)
	scoringConfigsRepo := repoMocks.NewMockScoringConfigs(ctrl)
	jobsRepo := repoMocks.NewMockJobs(ctrl)
	expRepo := repoMocks.NewMockExperiences(ctrl)
	projRepo := repoMocks.NewMockProjects(ctrl)
	projSkillsRepo := repoMocks.NewMockProjectSkills(ctrl)
	workflowsRepo := repoMocks.NewMockWorkflows(ctrl)
	eduRepo := repoMocks.NewMockEducations(ctrl)

	contract := &repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		UsersRepository:               usersRepo,
		UserJobsRepository:            userJobsRepo,
		ScoringConfigsRepository:      scoringConfigsRepo,
		JobsRepository:                jobsRepo,
		ExperiencesRepository:         expRepo,
		ProjectsRepository:            projRepo,
		ProjectSkillsRepository:       projSkillsRepo,
		WorkflowsRepository:           workflowsRepo,
		EducationsRepository:          eduRepo,
	}

	service := matching.NewService(contract)
	ctx := context.Background()
	evalID := uuid.New()
	caller := &models.Principal{ActorType: "worker", ID: "worker-1"}
	strongRec := string(constants.RecommendationStrongMatch)

	t.Run("returns unauthorized when caller is nil", func(t *testing.T) {
		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: false},
		}, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrUnauthorized))
		assert.Nil(t, res)
	})

	t.Run("returns error when evaluation not found", func(t *testing.T) {
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(nil, constants.ErrNotFound)

		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: false},
		}, caller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "not found")
	})

	t.Run("rejects non-successful evaluation status", func(t *testing.T) {
		eval := &models.MatchingEvaluation{
			ID:                  evalID,
			EvaluationStatus:    string(constants.MatchingStatusPending),
			RecommendationLabel: &strongRec,
		}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)

		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: false},
		}, caller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "evaluation must be successful")
	})

	t.Run("rejects ineligible recommendation label", func(t *testing.T) {
		lowRec := string(constants.RecommendationLowMatch)
		eval := &models.MatchingEvaluation{
			ID:                  evalID,
			EvaluationStatus:    string(constants.MatchingStatusSuccess),
			RecommendationLabel: &lowRec,
		}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)

		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: false},
		}, caller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "not eligible for resume generation")
	})

	t.Run("rejects already successful resume even with force=true", func(t *testing.T) {
		resStatus := string(constants.ResumeStatusSuccess)
		eval := &models.MatchingEvaluation{
			ID:                  evalID,
			EvaluationStatus:    string(constants.MatchingStatusSuccess),
			RecommendationLabel: &strongRec,
			ResumeStatus:        &resStatus,
		}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)

		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: true},
		}, caller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "regeneration not allowed")
	})

	t.Run("rejects pending resume when force is false", func(t *testing.T) {
		resStatus := string(constants.ResumeStatusPending)
		eval := &models.MatchingEvaluation{
			ID:                  evalID,
			EvaluationStatus:    string(constants.MatchingStatusSuccess),
			RecommendationLabel: &strongRec,
			ResumeStatus:        &resStatus,
		}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)

		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: false},
		}, caller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "already in progress")
	})

	t.Run("rejects incomplete onboarding", func(t *testing.T) {
		userJobID := uuid.New()
		userID := uuid.New()
		eval := &models.MatchingEvaluation{
			ID:                  evalID,
			UserJobID:           userJobID,
			EvaluationStatus:    string(constants.MatchingStatusSuccess),
			RecommendationLabel: &strongRec,
		}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: userID,
		}, nil)
		usersRepo.EXPECT().FindByID(ctx, userID.String()).Return(&models.User{
			ID:                    userID.String(),
			OnboardingCompletedAt: nil,
		}, nil)

		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: false},
		}, caller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "onboarding is incomplete")
	})

	t.Run("success queues resume and starts limiter work item", func(t *testing.T) {
		userJobID := uuid.New()
		userID := uuid.New()
		eval := &models.MatchingEvaluation{
			ID:                   evalID,
			UserJobID:            userJobID,
			EvaluationStatus:     string(constants.MatchingStatusSuccess),
			RecommendationLabel:  &strongRec,
			CandidateProfileData: json.RawMessage(`{"skills":["Go"],"experience":[]}`),
			JobProfileData:       json.RawMessage(`{"title":"Senior Engineer"}`),
			EvaluationData:       json.RawMessage(`{"summary":"Great fit"}`),
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: userID,
		}, nil)
		fullName := "Jane Doe"
		email := "jane@example.com"
		now := time.Now()
		usersRepo.EXPECT().FindByID(ctx, userID.String()).Return(&models.User{
			ID:                    userID.String(),
			FullName:              &fullName,
			Email:                 &email,
			OnboardingCompletedAt: &now,
		}, nil)
		eduRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.UserEducation{}, nil)
		expRepo.EXPECT().ListByUser(ctx, userID.String()).Return([]*models.Experience{}, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeResume, gomock.Any()).Return("wf-resume-1", nil)

		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: false},
		}, caller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
		assert.Equal(t, evalID, *res[0].ID)
	})
}

func TestUpdateEvaluationResume(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
	})
	ctx := context.Background()
	evalID := uuid.New()

	t.Run("rejects invalid status", func(t *testing.T) {
		_, err := service.UpdateEvaluationResume(ctx, evalID, matching.UpdateEvaluationResumeRequest{
			ResumeStatus: "invalid-status",
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrInvalidRequest))
	})

	t.Run("updates resume when valid", func(t *testing.T) {
		pdfKey := "resumes/1.pdf"
		jsonKey := "resumes/1.json"
		resStatus := string(constants.ResumeStatusSuccess)
		eval := &models.MatchingEvaluation{
			ID:           evalID,
			ResumeStatus: &resStatus,
		}
		evalRepo.EXPECT().UpdateResume(ctx, evalID, models.UpdateMatchingEvaluationResumeParams{
			ResumeStatus:  string(constants.ResumeStatusSuccess),
			ResumePDFKey:  &pdfKey,
			ResumeJSONKey: &jsonKey,
		}).Return(eval, nil)

		res, err := service.UpdateEvaluationResume(ctx, evalID, matching.UpdateEvaluationResumeRequest{
			ResumeStatus:  string(constants.ResumeStatusSuccess),
			ResumePDFKey:  &pdfKey,
			ResumeJSONKey: &jsonKey,
		})
		require.NoError(t, err)
		assert.Equal(t, evalID, res.ID)
	})
}

func TestGetResume(t *testing.T) {
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
	ownerID := uuid.New()

	t.Run("returns unauthorized when caller is nil", func(t *testing.T) {
		res, err := service.GetResume(ctx, evalID, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrUnauthorized))
		assert.Nil(t, res)
	})

	t.Run("returns not found when eval does not exist", func(t *testing.T) {
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(nil, constants.ErrNotFound)
		res, err := service.GetResume(ctx, evalID, &models.Principal{ActorType: "worker", ID: "worker-1"})
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrNotFound))
		assert.Nil(t, res)
	})

	t.Run("returns forbidden when clerk user is not owner", func(t *testing.T) {
		eval := &models.MatchingEvaluation{ID: evalID, UserJobID: userJobID}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: ownerID,
		}, nil)

		res, err := service.GetResume(ctx, evalID, &models.Principal{ActorType: "", ID: "different-user"})
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrForbidden))
		assert.Nil(t, res)
	})

	t.Run("returns evaluation when clerk user is owner", func(t *testing.T) {
		eval := &models.MatchingEvaluation{ID: evalID, UserJobID: userJobID}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: ownerID,
		}, nil)

		res, err := service.GetResume(ctx, evalID, &models.Principal{ActorType: "", ID: ownerID.String()})
		require.NoError(t, err)
		assert.Equal(t, eval, res)
	})
}
