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

func TestCreateEvaluations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)
	scoringConfigsRepo := repoMocks.NewMockScoringConfigs(ctrl)
	jobsRepo := repoMocks.NewMockJobs(ctrl)
	expRepo := repoMocks.NewMockExperiences(ctrl)
	projRepo := repoMocks.NewMockProjects(ctrl)
	projSkillsRepo := repoMocks.NewMockProjectSkills(ctrl)

	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		UserJobsRepository:            userJobsRepo,
		ScoringConfigsRepository:      scoringConfigsRepo,
		JobsRepository:                jobsRepo,
		ExperiencesRepository:         expRepo,
		ProjectsRepository:            projRepo,
		ProjectSkillsRepository:       projSkillsRepo,
	})
	ctx := context.Background()

	userID := uuid.New()
	slug := "test-job"
	job := &models.Job{
		ID:            101,
		Slug:          slug,
		Title:         "Backend Engineer",
		Company:       "Acme",
		Location:      "Remote",
		Status:        string(constants.StatusAnalyzed),
		AnalyzeResult: []byte(`{"family":"technology","metadata":{"model":"gpt-4","prompt_version":"v1"},"skills":["go"]}`),
	}
	userJob := &models.UserJob{ID: uuid.New(), UserID: userID, JobID: job.ID}
	scoringConfig := &models.ScoringConfig{
		ID:         uuid.New(),
		Family:     "technology",
		Categories: []models.CategoryConfig{{Name: "technical_skills", Weight: 1.0}},
	}

	// expectCandidate registers the profile-composition reads that run for each
	// resolved user before the evaluation is inserted.
	expectCandidate := func(uid string) {
		expRepo.EXPECT().ListByUser(ctx, uid).Return([]*models.Experience{}, nil)
		projRepo.EXPECT().ListByUser(ctx, uid).Return([]*models.Project{}, nil)
		projSkillsRepo.EXPECT().ListSkillsByUser(ctx, uid).Return([]*models.UserSkill{}, nil)
	}

	t.Run("job not found", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(nil, constants.ErrNotFound)

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "job not found")
	})

	t.Run("job not analyzed", func(t *testing.T) {
		unanalyzed := *job
		unanalyzed.AnalyzeResult = nil
		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: &unanalyzed}, nil)

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "is not analyzed")
	})

	t.Run("null analyze_result does not panic", func(t *testing.T) {
		nullAnalyze := *job
		nullAnalyze.AnalyzeResult = []byte(`null`)
		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: &nullAnalyze}, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error, "null analyze_result must be a per-item error, not a panic")
		assert.Contains(t, *res[0].Error, "not a JSON object")
	})

	t.Run("clerk caller does not own the job", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(false, nil)

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "job link not found")
	})

	t.Run("clerk caller supplying user_ids is rejected", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: job}, nil)

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{
			{Slug: slug, UserIDs: []string{uuid.NewString()}},
		}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "user_ids")
	})

	t.Run("api key caller with UserIDs creates one row per user", func(t *testing.T) {
		userA, userB := uuid.New(), uuid.New()
		jobA := &models.UserJob{ID: uuid.New(), UserID: userA, JobID: job.ID}
		jobB := &models.UserJob{ID: uuid.New(), UserID: userB, JobID: job.ID}
		evalA := &models.MatchingEvaluation{ID: uuid.New(), UserJobID: jobA.ID, EvaluationStatus: string(constants.MatchingStatusNew)}
		evalB := &models.MatchingEvaluation{ID: uuid.New(), UserJobID: jobB.ID, EvaluationStatus: string(constants.MatchingStatusNew)}

		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userA.String(), job.ID).Return(jobA, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userB.String(), job.ID).Return(jobB, nil)
		expectCandidate(userA.String())
		expectCandidate(userB.String())

		var seenUserJobs []uuid.UUID
		evalRepo.EXPECT().CreateIfAbsent(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, eval *models.MatchingEvaluation) (*models.MatchingEvaluation, bool, error) {
				assert.Equal(t, string(constants.MatchingStatusNew), eval.EvaluationStatus)
				seenUserJobs = append(seenUserJobs, eval.UserJobID)
				if eval.UserJobID == jobA.ID {
					return evalA, true, nil
				}
				return evalB, true, nil
			}).Times(2)

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{
			{Slug: slug, UserIDs: []string{userA.String(), userB.String()}},
		}, &models.Principal{ActorType: "api_key", ActorID: "key-1"})
		require.NoError(t, err)
		require.Len(t, res, 2)
		assert.Nil(t, res[0].Error)
		assert.Nil(t, res[1].Error)
		assert.Equal(t, evalA.ID, *res[0].ID)
		assert.Equal(t, evalB.ID, *res[1].ID)
		assert.Equal(t, string(constants.MatchingStatusNew), res[0].EvaluationStatus)
		assert.ElementsMatch(t, []uuid.UUID{jobA.ID, jobB.ID}, seenUserJobs)
	})

	t.Run("calling twice yields one row", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: job}, nil).Times(2)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil).Times(2)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil).Times(2)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil).Times(2)
		expectCandidate(userID.String())
		expectCandidate(userID.String())

		existing := &models.MatchingEvaluation{ID: uuid.New(), UserJobID: userJob.ID, EvaluationStatus: string(constants.MatchingStatusNew)}
		gomock.InOrder(
			evalRepo.EXPECT().CreateIfAbsent(ctx, gomock.Any()).Return(existing, true, nil),
			evalRepo.EXPECT().CreateIfAbsent(ctx, gomock.Any()).Return(existing, false, nil),
		)

		first, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, first, 1)
		second, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, second, 1)
		assert.Equal(t, existing.ID, *first[0].ID)
		assert.Equal(t, existing.ID, *second[0].ID)
	})

	t.Run("existing success row is returned unchanged", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil)
		expectCandidate(userID.String())

		successID := uuid.New()
		existing := &models.MatchingEvaluation{ID: successID, UserJobID: userJob.ID, EvaluationStatus: string(constants.MatchingStatusSuccess)}
		evalRepo.EXPECT().CreateIfAbsent(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, eval *models.MatchingEvaluation) (*models.MatchingEvaluation, bool, error) {
				assert.Equal(t, string(constants.MatchingStatusNew), eval.EvaluationStatus, "insert candidate must be at new status")
				return existing, false, nil
			})

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
		assert.Equal(t, successID, *res[0].ID)
		assert.Equal(t, string(constants.MatchingStatusSuccess), res[0].EvaluationStatus)
	})

	t.Run("stored job_profile_data is derived", func(t *testing.T) {
		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil)
		expectCandidate(userID.String())

		var stored map[string]interface{}
		evalRepo.EXPECT().CreateIfAbsent(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, eval *models.MatchingEvaluation) (*models.MatchingEvaluation, bool, error) {
				require.NoError(t, json.Unmarshal(eval.JobProfileData, &stored))
				return &models.MatchingEvaluation{ID: uuid.New(), UserJobID: userJob.ID, EvaluationStatus: string(constants.MatchingStatusNew)}, true, nil
			})

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)

		_, hasMetadata := stored["metadata"]
		assert.False(t, hasMetadata, "metadata must be stripped from the stored job profile")
		assert.Equal(t, "Backend Engineer", stored["title"])
		assert.Equal(t, "Acme", stored["company"])
		assert.Equal(t, "Remote", stored["location"])
		assert.Equal(t, "technology", stored["family"])
	})

	t.Run("defaults family to other when absent", func(t *testing.T) {
		noFamily := *job
		noFamily.AnalyzeResult = []byte(`{"skills":["go"]}`)
		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: &noFamily}, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "other").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil)
		expectCandidate(userID.String())

		var stored map[string]interface{}
		evalRepo.EXPECT().CreateIfAbsent(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, eval *models.MatchingEvaluation) (*models.MatchingEvaluation, bool, error) {
				require.NoError(t, json.Unmarshal(eval.JobProfileData, &stored))
				return &models.MatchingEvaluation{ID: uuid.New(), UserJobID: userJob.ID, EvaluationStatus: string(constants.MatchingStatusNew)}, true, nil
			})

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
		assert.Equal(t, "other", stored["family"])
	})

	t.Run("falls back to technology scoring config", func(t *testing.T) {
		otherFamily := *job
		otherFamily.AnalyzeResult = []byte(`{"family":"finance"}`)
		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: &otherFamily}, nil)
		userJobsRepo.EXPECT().IsOwned(ctx, userID.String(), job.ID).Return(true, nil)
		gomock.InOrder(
			scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "finance").Return(nil, errors.New("no finance config")),
			scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil),
		)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userID.String(), job.ID).Return(userJob, nil)
		expectCandidate(userID.String())

		evalRepo.EXPECT().CreateIfAbsent(ctx, gomock.Any()).Return(
			&models.MatchingEvaluation{ID: uuid.New(), UserJobID: userJob.ID, EvaluationStatus: string(constants.MatchingStatusNew)}, true, nil)

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, &models.Principal{ID: userID.String()})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Nil(t, res[0].Error)
	})
}

func TestCreateEvaluations_machine_caller_without_user_ids(t *testing.T) {
	ctx := context.Background()
	slug := "test-job"
	job := &models.Job{
		ID:            101,
		Slug:          slug,
		AnalyzeResult: []byte(`{"family":"technology"}`),
	}
	scoringConfig := &models.ScoringConfig{
		ID:         uuid.New(),
		Family:     "technology",
		Categories: []models.CategoryConfig{{Name: "technical_skills", Weight: 1.0}},
	}
	caller := &models.Principal{ActorType: "api_key", ActorID: "key-1"}

	newService := func(t *testing.T) (matching.Matching, *repoMocks.MockMatchingEvaluations, *repoMocks.MockUserJobs, *repoMocks.MockScoringConfigs, *repoMocks.MockJobs) {
		t.Helper()
		ctrl := gomock.NewController(t)
		evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
		userJobsRepo := repoMocks.NewMockUserJobs(ctrl)
		scoringConfigsRepo := repoMocks.NewMockScoringConfigs(ctrl)
		jobsRepo := repoMocks.NewMockJobs(ctrl)
		expRepo := repoMocks.NewMockExperiences(ctrl)
		projRepo := repoMocks.NewMockProjects(ctrl)
		projSkillsRepo := repoMocks.NewMockProjectSkills(ctrl)
		expRepo.EXPECT().ListByUser(gomock.Any(), gomock.Any()).Return([]*models.Experience{}, nil).AnyTimes()
		projRepo.EXPECT().ListByUser(gomock.Any(), gomock.Any()).Return([]*models.Project{}, nil).AnyTimes()
		projSkillsRepo.EXPECT().ListSkillsByUser(gomock.Any(), gomock.Any()).Return([]*models.UserSkill{}, nil).AnyTimes()
		service := matching.NewService(&repository.Contract{
			MatchingEvaluationsRepository: evalRepo,
			UserJobsRepository:            userJobsRepo,
			ScoringConfigsRepository:      scoringConfigsRepo,
			JobsRepository:                jobsRepo,
			ExperiencesRepository:         expRepo,
			ProjectsRepository:            projRepo,
			ProjectSkillsRepository:       projSkillsRepo,
		})
		return service, evalRepo, userJobsRepo, scoringConfigsRepo, jobsRepo
	}

	t.Run("resolves linked users and returns their user_id", func(t *testing.T) {
		service, evalRepo, userJobsRepo, scoringConfigsRepo, jobsRepo := newService(t)
		userA, userB := uuid.New(), uuid.New()
		jobA := &models.UserJob{ID: uuid.New(), UserID: userA, JobID: job.ID}
		jobB := &models.UserJob{ID: uuid.New(), UserID: userB, JobID: job.ID}
		evalA := &models.MatchingEvaluation{ID: uuid.New(), UserJobID: jobA.ID, EvaluationStatus: string(constants.MatchingStatusNew)}
		evalB := &models.MatchingEvaluation{ID: uuid.New(), UserJobID: jobB.ID, EvaluationStatus: string(constants.MatchingStatusNew)}

		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		scoringConfigsRepo.EXPECT().GetActiveByFamily(ctx, "technology").Return(scoringConfig, nil)
		userJobsRepo.EXPECT().GetUsersByJobID(ctx, job.ID).Return([]string{userA.String(), userB.String()}, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userA.String(), job.ID).Return(jobA, nil)
		userJobsRepo.EXPECT().GetByUserAndJob(ctx, userB.String(), job.ID).Return(jobB, nil)
		evalRepo.EXPECT().CreateIfAbsent(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, eval *models.MatchingEvaluation) (*models.MatchingEvaluation, bool, error) {
				if eval.UserJobID == jobA.ID {
					return evalA, true, nil
				}
				return evalB, true, nil
			}).Times(2)

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, caller)
		require.NoError(t, err)
		require.Len(t, res, 2)
		owners := map[uuid.UUID]string{
			*res[0].ID: res[0].UserID,
			*res[1].ID: res[1].UserID,
		}
		assert.Equal(t, map[uuid.UUID]string{
			evalA.ID: userA.String(),
			evalB.ID: userB.String(),
		}, owners)
	})

	t.Run("reports linked-user lookup failure", func(t *testing.T) {
		service, _, userJobsRepo, _, jobsRepo := newService(t)
		jobsRepo.EXPECT().GetBySlug(ctx, slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		userJobsRepo.EXPECT().GetUsersByJobID(ctx, job.ID).Return(nil, errors.New("db down"))

		res, err := service.CreateEvaluations(ctx, []models.CreateEvaluationItem{{Slug: slug}}, caller)
		require.NoError(t, err)
		require.Len(t, res, 1)
		require.NotNil(t, res[0].Error)
		assert.Contains(t, *res[0].Error, "get users by job id")
	})
}
