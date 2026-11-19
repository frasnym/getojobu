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

	t.Run("clerk user is denied when evaluation is not owned", func(t *testing.T) {
		userJobID := uuid.New()
		ownerID := uuid.New()
		eval := &models.MatchingEvaluation{ID: evalID, UserJobID: userJobID}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: ownerID,
		}, nil)

		clerkCaller := &models.Principal{ActorType: "", ID: "someone-else"}
		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: true},
		}, clerkCaller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "not authorized")
	})

	t.Run("clerk user is denied when evaluation has no user job link", func(t *testing.T) {
		eval := &models.MatchingEvaluation{ID: evalID}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, uuid.Nil).Return(nil, constants.ErrNotFound)

		clerkCaller := &models.Principal{ActorType: "", ID: "user-1"}
		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: true},
		}, clerkCaller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
	})

	t.Run("clerk user can retry when evaluation is owned", func(t *testing.T) {
		userJobID := uuid.New()
		userID := uuid.New()
		failedStatus := string(constants.ResumeStatusFailed)
		eval := &models.MatchingEvaluation{
			ID:                  evalID,
			UserJobID:           userJobID,
			EvaluationStatus:    string(constants.MatchingStatusSuccess),
			RecommendationLabel: &strongRec,
			EvaluationData:      json.RawMessage(`{"summary":"Great fit"}`),
			ResumeStatus:        &failedStatus,
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: userID,
		}, nil)
		// triggerResume fetches the evaluation and user job again.
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: userID,
		}, nil)
		now := time.Now()
		fullName := "Jane Doe"
		email := "jane@example.com"
		usersRepo.EXPECT().FindByID(ctx, userID.String()).Return(&models.User{
			ID:                    userID.String(),
			FullName:              &fullName,
			Email:                 &email,
			OnboardingCompletedAt: &now,
		}, nil)
		evalRepo.EXPECT().UpdateResume(ctx, evalID, models.UpdateMatchingEvaluationResumeParams{
			ResumeStatus: string(constants.ResumeStatusQueuedRetry),
		}).Return(eval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeResume, gomock.Any()).Return("wf-resume-1", nil)

		clerkCaller := &models.Principal{ActorType: "", ID: userID.String()}
		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: true},
		}, clerkCaller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.Nil(t, res[0].Error)
		assert.Equal(t, evalID, *res[0].ID)
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

	t.Run("allows any recommendation label to generate resume", func(t *testing.T) {
		userJobID := uuid.New()
		userID := uuid.New()
		lowRec := string(constants.RecommendationLowMatch)
		eval := &models.MatchingEvaluation{
			ID:                  evalID,
			UserJobID:           userJobID,
			EvaluationStatus:    string(constants.MatchingStatusSuccess),
			RecommendationLabel: &lowRec,
			EvaluationData:      json.RawMessage(`{"summary":"Great fit"}`),
		}

		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: userID,
		}, nil)
		// triggerResume fetches the evaluation and user job again.
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: userID,
		}, nil)
		now := time.Now()
		fullName := "Jane Doe"
		email := "jane@example.com"
		usersRepo.EXPECT().FindByID(ctx, userID.String()).Return(&models.User{
			ID:                    userID.String(),
			FullName:              &fullName,
			Email:                 &email,
			OnboardingCompletedAt: &now,
		}, nil)
		evalRepo.EXPECT().UpdateResume(ctx, evalID, models.UpdateMatchingEvaluationResumeParams{
			ResumeStatus: string(constants.ResumeStatusQueued),
		}).Return(eval, nil)
		workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, gomock.Any(), models.LimiterJobTypeResume, gomock.Any()).Return("wf-resume-1", nil)

		clerkCaller := &models.Principal{ActorType: "", ID: userID.String()}
		res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{
			{ID: evalID, Force: false},
		}, clerkCaller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.Nil(t, res[0].Error)
		assert.Equal(t, evalID, *res[0].ID)
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
			ID:                  evalID,
			UserJobID:           userJobID,
			EvaluationStatus:    string(constants.MatchingStatusSuccess),
			RecommendationLabel: &strongRec,
			EvaluationData:      json.RawMessage(`{"summary":"Great fit"}`),
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
		evalRepo.EXPECT().UpdateResume(ctx, evalID, models.UpdateMatchingEvaluationResumeParams{
			ResumeStatus: string(constants.ResumeStatusQueued),
		}).Return(eval, nil)
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

func TestBulkQueueGenerateResume_ForceQueuedGuard(t *testing.T) {
	ctx := context.Background()
	caller := &models.Principal{ActorType: "worker", ID: "worker-1"}
	rec := string(constants.RecommendationStrongMatch)

	newService := func(t *testing.T) (matching.Matching, *repoMocks.MockMatchingEvaluations, *repoMocks.MockUserJobs, *repoMocks.MockUsers, *repoMocks.MockWorkflows) {
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
		return service, evalRepo, userJobsRepo, usersRepo, workflowsRepo
	}

	queuedEval := func(id, userJobID uuid.UUID, status string, age time.Duration) *models.MatchingEvaluation {
		resumeStatus := status
		return &models.MatchingEvaluation{
			ID:                  id,
			UserJobID:           userJobID,
			EvaluationStatus:    string(constants.MatchingStatusSuccess),
			RecommendationLabel: &rec,
			ResumeStatus:        &resumeStatus,
			EvaluationData:      json.RawMessage(`{"summary":"Great fit"}`),
			UpdatedAt:           time.Now().Add(-age),
		}
	}

	for _, status := range []string{string(constants.ResumeStatusQueued), string(constants.ResumeStatusQueuedRetry)} {
		t.Run(status+"_recent_rejected", func(t *testing.T) {
			service, evalRepo, _, _, workflowsRepo := newService(t)
			id, userJobID := uuid.New(), uuid.New()
			evalRepo.EXPECT().GetByID(ctx, id).Return(queuedEval(id, userJobID, status, 10*time.Minute), nil)
			workflowsRepo.EXPECT().StartLimiterWorkItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{{ID: id, Force: true}}, caller)
			require.NoError(t, err)
			require.Len(t, res, 1)
			require.NotNil(t, res[0].Error)
			assert.Contains(t, *res[0].Error, "less than 30 minutes")
		})

		t.Run(status+"_stale_running_rejected", func(t *testing.T) {
			service, evalRepo, _, _, workflowsRepo := newService(t)
			id, userJobID := uuid.New(), uuid.New()
			evalRepo.EXPECT().GetByID(ctx, id).Return(queuedEval(id, userJobID, status, 45*time.Minute), nil)
			workflowsRepo.EXPECT().IsWorkflowRunning(ctx, "resume-gen-"+id.String()).Return(true, nil)
			workflowsRepo.EXPECT().StartLimiterWorkItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{{ID: id, Force: true}}, caller)
			require.NoError(t, err)
			require.Len(t, res, 1)
			require.NotNil(t, res[0].Error)
			assert.Contains(t, *res[0].Error, "currently running")
		})

		t.Run(status+"_stale_not_running_enqueues", func(t *testing.T) {
			service, evalRepo, userJobsRepo, usersRepo, workflowsRepo := newService(t)
			id, userJobID, userID := uuid.New(), uuid.New(), uuid.New()
			eval := queuedEval(id, userJobID, status, 45*time.Minute)
			now := time.Now()
			name := "Jane Doe"
			evalRepo.EXPECT().GetByID(ctx, id).Return(eval, nil)
			workflowsRepo.EXPECT().IsWorkflowRunning(ctx, "resume-gen-"+id.String()).Return(false, nil)
			userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{ID: userJobID, UserID: userID}, nil)
			usersRepo.EXPECT().FindByID(ctx, userID.String()).Return(&models.User{
				ID: userID.String(), FullName: &name, OnboardingCompletedAt: &now,
			}, nil)
			evalRepo.EXPECT().UpdateResume(ctx, id, models.UpdateMatchingEvaluationResumeParams{
				ResumeStatus: string(constants.ResumeStatusQueuedRetry),
			}).Return(eval, nil)
			workflowsRepo.EXPECT().StartLimiterWorkItem(ctx, "resume-gen-"+id.String(), models.LimiterJobTypeResume, gomock.Any()).Return("wf-resume", nil)

			res, err := service.BulkQueueGenerateResume(ctx, []models.BulkGenerateResumeItem{{ID: id, Force: true}}, caller)
			require.NoError(t, err)
			require.Len(t, res, 1)
			assert.Nil(t, res[0].Error)
		})
	}
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

func TestDownloadResume(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	usersRepo := repoMocks.NewMockUsers(ctrl)
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)
	storageRepo := repoMocks.NewMockStorage(ctrl)

	contract := &repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		UsersRepository:               usersRepo,
		UserJobsRepository:            userJobsRepo,
		StorageRepository:             storageRepo,
	}

	service := matching.NewService(contract)
	ctx := context.Background()
	evalID := uuid.New()
	userJobID := uuid.New()
	ownerID := uuid.New()

	pdfKey := "123/456/resume.pdf"
	statusSuccess := string(constants.ResumeStatusSuccess)
	statusPending := string(constants.ResumeStatusPending)

	t.Run("returns unauthorized when caller is nil", func(t *testing.T) {
		res, err := service.DownloadResume(ctx, evalID, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrUnauthorized))
		assert.Nil(t, res)
	})

	t.Run("returns error when resume is not success", func(t *testing.T) {
		eval := &models.MatchingEvaluation{ID: evalID, ResumeStatus: &statusPending}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)

		res, err := service.DownloadResume(ctx, evalID, &models.Principal{ActorType: "worker", ID: "worker-1"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resume is not available")
		assert.Nil(t, res)
	})

	t.Run("returns error when pdf key is missing", func(t *testing.T) {
		eval := &models.MatchingEvaluation{ID: evalID, ResumeStatus: &statusSuccess, ResumePDFKey: nil}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)

		res, err := service.DownloadResume(ctx, evalID, &models.Principal{ActorType: "worker", ID: "worker-1"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resume pdf key is missing")
		assert.Nil(t, res)
	})

	t.Run("returns forbidden when caller is not owner", func(t *testing.T) {
		eval := &models.MatchingEvaluation{
			ID:           evalID,
			UserJobID:    userJobID,
			ResumeStatus: &statusSuccess,
			ResumePDFKey: &pdfKey,
		}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: ownerID,
		}, nil)

		res, err := service.DownloadResume(ctx, evalID, &models.Principal{ActorType: "", ID: "someone-else"})
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrForbidden))
		assert.Nil(t, res)
	})

	t.Run("returns presigned download url with candidate name and job title when owner calls", func(t *testing.T) {
		// Real job analysis payloads have no "title" field; the position comes
		// from the jobs table, not from job_profile_data.
		eval := &models.MatchingEvaluation{
			ID:           evalID,
			UserJobID:    userJobID,
			ResumeStatus: &statusSuccess,
			ResumePDFKey: &pdfKey,
		}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: ownerID,
		}, nil)
		userName := "Jane Doe"
		usersRepo.EXPECT().FindByID(ctx, ownerID.String()).Return(&models.User{
			ID:       ownerID.String(),
			FullName: &userName,
		}, nil)
		evalRepo.EXPECT().GetLatestByUserJobID(ctx, ownerID, userJobID).Return(&models.EvaluationWithJob{
			Job: &models.Job{Title: "Senior Go Engineer"},
		}, nil)

		expectedURL := "https://r2.example.com/download-resume"
		storageRepo.EXPECT().GetPresignedDownloadURL(ctx, pdfKey, "Jane_Doe_Resume_Senior-Go-Engineer.pdf", 15*time.Minute).Return(expectedURL, nil)

		res, err := service.DownloadResume(ctx, evalID, &models.Principal{ActorType: "", ID: ownerID.String()})
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, expectedURL, res.DownloadURL)
		assert.Equal(t, "Jane_Doe_Resume_Senior-Go-Engineer.pdf", res.Filename)
		assert.False(t, res.ExpiresAt.IsZero())
	})

	t.Run("falls back to Resume.pdf when name and title are missing", func(t *testing.T) {
		eval := &models.MatchingEvaluation{
			ID:           evalID,
			UserJobID:    userJobID,
			ResumeStatus: &statusSuccess,
			ResumePDFKey: &pdfKey,
		}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: ownerID,
		}, nil)
		usersRepo.EXPECT().FindByID(ctx, ownerID.String()).Return(&models.User{
			ID:       ownerID.String(),
			FullName: nil,
		}, nil)
		evalRepo.EXPECT().GetLatestByUserJobID(ctx, ownerID, userJobID).Return(&models.EvaluationWithJob{
			Job: &models.Job{Title: ""},
		}, nil)

		expectedURL := "https://r2.example.com/download-resume"
		storageRepo.EXPECT().GetPresignedDownloadURL(ctx, pdfKey, "Resume.pdf", 15*time.Minute).Return(expectedURL, nil)

		res, err := service.DownloadResume(ctx, evalID, &models.Principal{ActorType: "", ID: ownerID.String()})
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "Resume.pdf", res.Filename)
	})

	t.Run("uses name and Resume when only job title is missing", func(t *testing.T) {
		eval := &models.MatchingEvaluation{
			ID:           evalID,
			UserJobID:    userJobID,
			ResumeStatus: &statusSuccess,
			ResumePDFKey: &pdfKey,
		}
		evalRepo.EXPECT().GetByID(ctx, evalID).Return(eval, nil)
		userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
			ID: userJobID, UserID: ownerID,
		}, nil)
		userName := "Jane Doe"
		usersRepo.EXPECT().FindByID(ctx, ownerID.String()).Return(&models.User{
			ID:       ownerID.String(),
			FullName: &userName,
		}, nil)
		evalRepo.EXPECT().GetLatestByUserJobID(ctx, ownerID, userJobID).Return(&models.EvaluationWithJob{
			Job: &models.Job{Title: ""},
		}, nil)

		expectedURL := "https://r2.example.com/download-resume"
		storageRepo.EXPECT().GetPresignedDownloadURL(ctx, pdfKey, "Jane_Doe_Resume.pdf", 15*time.Minute).Return(expectedURL, nil)

		res, err := service.DownloadResume(ctx, evalID, &models.Principal{ActorType: "", ID: ownerID.String()})
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "Jane_Doe_Resume.pdf", res.Filename)
	})
}
