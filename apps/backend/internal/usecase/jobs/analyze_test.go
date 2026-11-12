package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repoMocks "github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

func ptrString(s string) *string {
	return &s
}

func TestGenerateWorkflowID(t *testing.T) {
	t.Run("default_without_force", func(t *testing.T) {
		id := GenerateWorkflowID(42, false)
		assert.Equal(t, "job-analyze-42", id)
	})

	t.Run("with_force", func(t *testing.T) {
		id1 := GenerateWorkflowID(42, true)
		time.Sleep(2 * time.Millisecond)
		id2 := GenerateWorkflowID(42, true)
		assert.True(t, strings.HasPrefix(id1, "job-analyze-42-"))
		assert.True(t, strings.HasPrefix(id2, "job-analyze-42-"))
		assert.NotEqual(t, id1, id2, "each force run should have a unique epoch suffix")
	})
}

func TestQueueAnalysisBulk(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockJobs(ctrl)
	mockUserJobs := repoMocks.NewMockUserJobs(ctrl)
	mockWorkflows := repoMocks.NewMockWorkflows(ctrl)

	t.Run("empty_items_returns_nil", func(t *testing.T) {
		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		res, err := svc.QueueAnalysisBulk(context.Background(), nil, nil)
		require.NoError(t, err)
		assert.Nil(t, res)
	})

	t.Run("works_when_status_new_force_false", func(t *testing.T) {
		slug := "job-new"
		job := &models.Job{
			ID:          101,
			Slug:        slug,
			Status:      string(constants.StatusNew),
			Description: ptrString("Software Engineer at Acme"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug).Return(job, nil)

		expectedInput := models.JobAnalyzeWorkflowInput{
			JobDescription: "Software Engineer at Acme",
			JobID:          "101",
			Slug:           slug,
		}
		mockWorkflows.EXPECT().StartLimiterWorkItem(gomock.Any(), "job-analyze-101", models.LimiterJobTypeAnalyze, expectedInput).Return("job-analyze-101", nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		// API key principal caller
		caller := &models.Principal{ActorType: "cron", ActorID: "cron-scrape"}
		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}

		ids, err := svc.QueueAnalysisBulk(context.Background(), items, caller)
		require.NoError(t, err)
		require.Len(t, ids, 1)
		assert.Equal(t, "job-analyze-101", ids[0])
	})

	t.Run("works_when_status_failed_force_true", func(t *testing.T) {
		slug := "job-failed"
		job := &models.Job{
			ID:          102,
			Slug:        slug,
			Status:      string(constants.StatusFailed),
			Description: ptrString("Software Engineer"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug).Return(job, nil)

		expectedInput := models.JobAnalyzeWorkflowInput{
			JobDescription: "Software Engineer",
			JobID:          "102",
			Slug:           slug,
		}
		mockWorkflows.EXPECT().StartLimiterWorkItem(gomock.Any(), gomock.Cond(func(x any) bool {
			id, ok := x.(string)
			return ok && strings.HasPrefix(id, "job-analyze-102-")
		}), models.LimiterJobTypeAnalyze, expectedInput).DoAndReturn(func(_ context.Context, wid string, _ models.LimiterJobType, _ any) (string, error) {
			return wid, nil
		})

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		caller := &models.Principal{ActorType: "worker", ActorID: "getojobu-worker"}
		items := []models.QueueAnalysisItem{{Slug: slug, Force: true}}

		ids, err := svc.QueueAnalysisBulk(context.Background(), items, caller)
		require.NoError(t, err)
		require.Len(t, ids, 1)
		assert.True(t, strings.HasPrefix(ids[0], "job-analyze-102-"))
	})

	t.Run("rejects_when_status_failed_force_false", func(t *testing.T) {
		slug := "job-failed-no-force"
		job := &models.Job{
			ID:          103,
			Slug:        slug,
			Status:      string(constants.StatusFailed),
			Description: ptrString("Software Engineer"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug).Return(job, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}
		_, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrInvalidTransition))
		assert.Contains(t, err.Error(), "force must be true")
	})

	t.Run("rejects_when_status_analyzed", func(t *testing.T) {
		slug := "job-analyzed"
		job := &models.Job{
			ID:          104,
			Slug:        slug,
			Status:      string(constants.StatusAnalyzed),
			Description: ptrString("Software Engineer"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug).Return(job, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}
		_, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrInvalidTransition))
		assert.Contains(t, err.Error(), "cannot be queued for analysis with status \"analyzed\"")
	})

	t.Run("rejects_when_no_description", func(t *testing.T) {
		slug := "job-no-desc"
		job := &models.Job{
			ID:          105,
			Slug:        slug,
			Status:      string(constants.StatusNew),
			Description: nil,
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug).Return(job, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}
		_, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrInvalidTransition))
		assert.Contains(t, err.Error(), "has no description to analyze")
	})

	t.Run("owner_only_for_clerk", func(t *testing.T) {
		slug := "job-foreign"
		job := &models.Job{
			ID:          106,
			Slug:        slug,
			Status:      string(constants.StatusNew),
			Description: ptrString("Job"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug).Return(job, nil)
		mockUserJobs.EXPECT().IsOwned(gomock.Any(), "user-alice", int64(106)).Return(false, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		clerkCaller := &models.Principal{ID: "user-alice", ActorType: ""}
		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}

		_, err := svc.QueueAnalysisBulk(context.Background(), items, clerkCaller)
		require.Error(t, err)
		assert.True(t, errors.Is(err, constants.ErrNotFound))
	})

	t.Run("workflows_repository_error_propagates", func(t *testing.T) {
		slug := "job-error"
		job := &models.Job{
			ID:          107,
			Slug:        slug,
			Status:      string(constants.StatusNew),
			Description: ptrString("Software Engineer"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug).Return(job, nil)

		expectedInput := models.JobAnalyzeWorkflowInput{
			JobDescription: "Software Engineer",
			JobID:          "107",
			Slug:           slug,
		}
		mockWorkflows.EXPECT().StartLimiterWorkItem(gomock.Any(), "job-analyze-107", models.LimiterJobTypeAnalyze, expectedInput).Return("", errors.New("temporal connection error"))

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}
		_, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "temporal connection error")
	})

	t.Run("workflows_not_configured", func(t *testing.T) {
		slug := "job-no-wf"
		job := &models.Job{
			ID:          108,
			Slug:        slug,
			Status:      string(constants.StatusNew),
			Description: ptrString("Software Engineer"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug).Return(job, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: nil,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}
		_, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflows repository not configured")
	})
}
