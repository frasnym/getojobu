package jobs

import (
	"context"
	"errors"
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
		id := GenerateWorkflowID(42, true)
		assert.Equal(t, "job-analyze-42", id)
	})
}

func TestQueueAnalysisBulk(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockJobs(ctrl)
	mockUserJobs := repoMocks.NewMockUserJobs(ctrl)
	mockWorkflows := repoMocks.NewMockWorkflows(ctrl)

	t.Run("empty_items_returns_empty_result", func(t *testing.T) {
		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		res, err := svc.QueueAnalysisBulk(context.Background(), nil, nil)
		require.NoError(t, err)
		assert.Empty(t, res.WorkflowIDs)
		assert.Empty(t, res.Failed)
	})

	t.Run("works_when_status_new_force_false", func(t *testing.T) {
		slug := "job-new"
		job := &models.Job{
			ID:          101,
			Slug:        slug,
			Status:      string(constants.StatusNew),
			Description: ptrString("Software Engineer at Acme"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		mockRepo.EXPECT().UpdateStatus(gomock.Any(), slug, "queued", (*string)(nil), (*string)(nil), "cron", "cron-scrape").Return(job, nil)

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

		res, err := svc.QueueAnalysisBulk(context.Background(), items, caller)
		require.NoError(t, err)
		require.Len(t, res.WorkflowIDs, 1)
		assert.Equal(t, "job-analyze-101", res.WorkflowIDs[0])
		assert.Empty(t, res.Failed)
	})

	t.Run("works_when_status_queued_retry", func(t *testing.T) {
		slug := "job-queued-retry"
		job := &models.Job{
			ID:          1011,
			Slug:        slug,
			Status:      string(constants.StatusQueuedRetry),
			Description: ptrString("Software Engineer at Acme"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		mockRepo.EXPECT().UpdateStatus(gomock.Any(), slug, "queued_retry", (*string)(nil), (*string)(nil), "", "").Return(job, nil)

		expectedInput := models.JobAnalyzeWorkflowInput{
			JobDescription: "Software Engineer at Acme",
			JobID:          "1011",
			Slug:           slug,
		}
		mockWorkflows.EXPECT().StartLimiterWorkItem(gomock.Any(), "job-analyze-1011", models.LimiterJobTypeAnalyze, expectedInput).Return("job-analyze-1011", nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: true}}

		res, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.NoError(t, err)
		require.Len(t, res.WorkflowIDs, 1)
		assert.Equal(t, "job-analyze-1011", res.WorkflowIDs[0])
		assert.Empty(t, res.Failed)
	})

	t.Run("works_when_status_failed_force_true", func(t *testing.T) {
		slug := "job-failed"
		job := &models.Job{
			ID:          102,
			Slug:        slug,
			Status:      string(constants.StatusFailed),
			Description: ptrString("Software Engineer"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		mockRepo.EXPECT().UpdateStatus(gomock.Any(), slug, "queued_retry", (*string)(nil), (*string)(nil), "worker", "getojobu-worker").Return(job, nil)

		expectedInput := models.JobAnalyzeWorkflowInput{
			JobDescription: "Software Engineer",
			JobID:          "102",
			Slug:           slug,
		}
		mockWorkflows.EXPECT().StartLimiterWorkItem(gomock.Any(), "job-analyze-102", models.LimiterJobTypeAnalyze, expectedInput).Return("job-analyze-102", nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		caller := &models.Principal{ActorType: "worker", ActorID: "getojobu-worker"}
		items := []models.QueueAnalysisItem{{Slug: slug, Force: true}}

		res, err := svc.QueueAnalysisBulk(context.Background(), items, caller)
		require.NoError(t, err)
		require.Len(t, res.WorkflowIDs, 1)
		assert.Equal(t, "job-analyze-102", res.WorkflowIDs[0])
		assert.Empty(t, res.Failed)
	})

	t.Run("rejects_when_status_failed_force_false", func(t *testing.T) {
		slug := "job-failed-no-force"
		job := &models.Job{
			ID:          103,
			Slug:        slug,
			Status:      string(constants.StatusFailed),
			Description: ptrString("Software Engineer"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}
		res, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.NoError(t, err)
		assert.Empty(t, res.WorkflowIDs)
		require.Len(t, res.Failed, 1)
		assert.Equal(t, slug, res.Failed[0].Slug)
		assert.Contains(t, res.Failed[0].Error, "force must be true")
	})

	t.Run("rejects_when_status_analyze_pending_force_false", func(t *testing.T) {
		slug := "job-pending-no-force"
		job := &models.Job{
			ID:          1031,
			Slug:        slug,
			Status:      string(constants.StatusAnalyzePending),
			Description: ptrString("Software Engineer"),
			UpdatedAt:   time.Now().Add(-45 * time.Minute),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}
		res, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.NoError(t, err)
		assert.Empty(t, res.WorkflowIDs)
		require.Len(t, res.Failed, 1)
		assert.Equal(t, slug, res.Failed[0].Slug)
		assert.Contains(t, res.Failed[0].Error, "force must be true")
	})

	t.Run("rejects_when_status_analyze_pending_updated_recently", func(t *testing.T) {
		slug := "job-pending-recent"
		job := &models.Job{
			ID:          1032,
			Slug:        slug,
			Status:      string(constants.StatusAnalyzePending),
			Description: ptrString("Software Engineer"),
			UpdatedAt:   time.Now().Add(-10 * time.Minute),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: true}}
		res, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.NoError(t, err)
		assert.Empty(t, res.WorkflowIDs)
		require.Len(t, res.Failed, 1)
		assert.Equal(t, slug, res.Failed[0].Slug)
		assert.Contains(t, res.Failed[0].Error, "less than 30 minutes")
	})

	t.Run("rejects_when_status_analyze_pending_workflow_still_running", func(t *testing.T) {
		slug := "job-pending-running"
		job := &models.Job{
			ID:          1033,
			Slug:        slug,
			Status:      string(constants.StatusAnalyzePending),
			Description: ptrString("Software Engineer"),
			UpdatedAt:   time.Now().Add(-45 * time.Minute),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		mockWorkflows.EXPECT().IsWorkflowRunning(gomock.Any(), "job-analyze-1033").Return(true, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: true}}
		res, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.NoError(t, err)
		assert.Empty(t, res.WorkflowIDs)
		require.Len(t, res.Failed, 1)
		assert.Equal(t, slug, res.Failed[0].Slug)
		assert.Contains(t, res.Failed[0].Error, "currently running")
	})

	t.Run("allows_retry_when_status_analyze_pending_stale_and_workflow_finished", func(t *testing.T) {
		slug := "job-stale-retry-ok"
		job := &models.Job{
			ID:          1034,
			Slug:        slug,
			Status:      string(constants.StatusAnalyzePending),
			Description: ptrString("Software Engineer"),
			UpdatedAt:   time.Now().Add(-45 * time.Minute),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		mockWorkflows.EXPECT().IsWorkflowRunning(gomock.Any(), "job-analyze-1034").Return(false, nil)
		mockRepo.EXPECT().UpdateStatus(gomock.Any(), slug, "queued_retry", (*string)(nil), (*string)(nil), "", "").Return(job, nil)
		expectedInput := models.JobAnalyzeWorkflowInput{
			JobDescription: "Software Engineer",
			JobID:          "1034",
			Slug:           slug,
		}
		mockWorkflows.EXPECT().StartLimiterWorkItem(gomock.Any(), "job-analyze-1034", models.LimiterJobTypeAnalyze, expectedInput).Return("job-analyze-1034", nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: true}}
		res, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.NoError(t, err)
		require.Len(t, res.WorkflowIDs, 1)
		assert.Equal(t, "job-analyze-1034", res.WorkflowIDs[0])
		assert.Empty(t, res.Failed)
	})

	t.Run("rejects_when_status_analyzed", func(t *testing.T) {
		slug := "job-analyzed"
		job := &models.Job{
			ID:          104,
			Slug:        slug,
			Status:      string(constants.StatusAnalyzed),
			Description: ptrString("Software Engineer"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}
		res, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.NoError(t, err)
		assert.Empty(t, res.WorkflowIDs)
		require.Len(t, res.Failed, 1)
		assert.Equal(t, slug, res.Failed[0].Slug)
		assert.Contains(t, res.Failed[0].Error, "cannot be queued for analysis with status \"analyzed\"")
	})

	t.Run("rejects_when_no_description", func(t *testing.T) {
		slug := "job-no-desc"
		job := &models.Job{
			ID:          105,
			Slug:        slug,
			Status:      string(constants.StatusNew),
			Description: nil,
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}
		res, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.NoError(t, err)
		assert.Empty(t, res.WorkflowIDs)
		require.Len(t, res.Failed, 1)
		assert.Equal(t, slug, res.Failed[0].Slug)
		assert.Equal(t, `job "job-no-desc" has no description to analyze`, res.Failed[0].Error)
		assert.NotContains(t, res.Failed[0].Error, "invalid status transition")
	})

	t.Run("owner_only_for_clerk", func(t *testing.T) {
		slug := "job-foreign"
		job := &models.Job{
			ID:          106,
			Slug:        slug,
			Status:      string(constants.StatusNew),
			Description: ptrString("Job"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		mockUserJobs.EXPECT().IsOwned(gomock.Any(), "user-alice", int64(106)).Return(false, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: mockWorkflows,
		}

		clerkCaller := &models.Principal{ID: "user-alice", ActorType: ""}
		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}

		res, err := svc.QueueAnalysisBulk(context.Background(), items, clerkCaller)
		require.NoError(t, err)
		assert.Empty(t, res.WorkflowIDs)
		require.Len(t, res.Failed, 1)
		assert.Equal(t, slug, res.Failed[0].Slug)
		assert.Contains(t, res.Failed[0].Error, "not found")
	})

	t.Run("workflows_repository_error_propagates", func(t *testing.T) {
		slug := "job-error"
		job := &models.Job{
			ID:          107,
			Slug:        slug,
			Status:      string(constants.StatusNew),
			Description: ptrString("Software Engineer"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
		mockRepo.EXPECT().UpdateStatus(gomock.Any(), slug, "queued", (*string)(nil), (*string)(nil), "", "").Return(job, nil)

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
		res, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.NoError(t, err)
		assert.Empty(t, res.WorkflowIDs)
		require.Len(t, res.Failed, 1)
		assert.Equal(t, slug, res.Failed[0].Slug)
		assert.Contains(t, res.Failed[0].Error, "temporal connection error")
	})

	t.Run("workflows_not_configured", func(t *testing.T) {
		slug := "job-no-wf"
		job := &models.Job{
			ID:          108,
			Slug:        slug,
			Status:      string(constants.StatusNew),
			Description: ptrString("Software Engineer"),
		}

		mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)

		svc := &jobService{
			repo:      mockRepo,
			userJobs:  mockUserJobs,
			workflows: nil,
		}

		items := []models.QueueAnalysisItem{{Slug: slug, Force: false}}
		res, err := svc.QueueAnalysisBulk(context.Background(), items, nil)
		require.NoError(t, err)
		assert.Empty(t, res.WorkflowIDs)
		require.Len(t, res.Failed, 1)
		assert.Equal(t, slug, res.Failed[0].Slug)
		assert.Contains(t, res.Failed[0].Error, "workflows repository not configured")
	})
}

func TestQueueAnalysisBulk_continues_after_no_description(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockJobs(ctrl)
	mockUserJobs := repoMocks.NewMockUserJobs(ctrl)
	mockWorkflows := repoMocks.NewMockWorkflows(ctrl)

	badSlug := "job-no-desc"
	okSlug := "job-ok"
	okJob := &models.Job{
		ID:          201,
		Slug:        okSlug,
		Status:      string(constants.StatusNew),
		Description: ptrString("Software Engineer"),
	}

	mockRepo.EXPECT().GetBySlug(gomock.Any(), badSlug, "").Return(&models.JobWithOwnership{
		Job: &models.Job{ID: 200, Slug: badSlug, Status: string(constants.StatusNew)},
	}, nil)
	mockRepo.EXPECT().GetBySlug(gomock.Any(), okSlug, "").Return(&models.JobWithOwnership{Job: okJob}, nil)
	mockRepo.EXPECT().UpdateStatus(gomock.Any(), okSlug, "queued", (*string)(nil), (*string)(nil), "", "").Return(okJob, nil)
	mockWorkflows.EXPECT().StartLimiterWorkItem(gomock.Any(), "job-analyze-201", models.LimiterJobTypeAnalyze, models.JobAnalyzeWorkflowInput{
		JobDescription: "Software Engineer", JobID: "201", Slug: okSlug,
	}).Return("job-analyze-201", nil)

	svc := &jobService{repo: mockRepo, userJobs: mockUserJobs, workflows: mockWorkflows}
	res, err := svc.QueueAnalysisBulk(context.Background(), []models.QueueAnalysisItem{
		{Slug: badSlug}, {Slug: okSlug},
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, []string{"job-analyze-201"}, res.WorkflowIDs)
	require.Len(t, res.Failed, 1)
	assert.Equal(t, badSlug, res.Failed[0].Slug)
	assert.Equal(t, `job "job-no-desc" has no description to analyze`, res.Failed[0].Error)
}

func TestQueueAnalysisBulk_continues_after_already_queued(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockJobs(ctrl)
	mockUserJobs := repoMocks.NewMockUserJobs(ctrl)
	mockWorkflows := repoMocks.NewMockWorkflows(ctrl)

	badSlug := "job-already-queued"
	okSlug := "job-ok"
	okJob := &models.Job{
		ID:          203,
		Slug:        okSlug,
		Status:      string(constants.StatusNew),
		Description: ptrString("Software Engineer"),
	}

	mockRepo.EXPECT().GetBySlug(gomock.Any(), badSlug, "").Return(&models.JobWithOwnership{
		Job: &models.Job{ID: 202, Slug: badSlug, Status: string(constants.StatusQueued), Description: ptrString("Software Engineer")},
	}, nil)
	mockRepo.EXPECT().GetBySlug(gomock.Any(), okSlug, "").Return(&models.JobWithOwnership{Job: okJob}, nil)
	mockRepo.EXPECT().UpdateStatus(gomock.Any(), okSlug, "queued", (*string)(nil), (*string)(nil), "", "").Return(okJob, nil)
	mockWorkflows.EXPECT().StartLimiterWorkItem(gomock.Any(), "job-analyze-203", models.LimiterJobTypeAnalyze, models.JobAnalyzeWorkflowInput{
		JobDescription: "Software Engineer", JobID: "203", Slug: okSlug,
	}).Return("job-analyze-203", nil)

	svc := &jobService{repo: mockRepo, userJobs: mockUserJobs, workflows: mockWorkflows}
	res, err := svc.QueueAnalysisBulk(context.Background(), []models.QueueAnalysisItem{
		{Slug: badSlug}, {Slug: okSlug},
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, []string{"job-analyze-203"}, res.WorkflowIDs)
	require.Len(t, res.Failed, 1)
	assert.Equal(t, badSlug, res.Failed[0].Slug)
	assert.Equal(t, `job "job-already-queued" is already queued for analysis`, res.Failed[0].Error)
}

func TestQueueAnalysisBulk_continues_after_invalid_status(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockJobs(ctrl)
	mockUserJobs := repoMocks.NewMockUserJobs(ctrl)
	mockWorkflows := repoMocks.NewMockWorkflows(ctrl)

	badSlug := "job-analyzed"
	okSlug := "job-ok"
	okJob := &models.Job{
		ID:          205,
		Slug:        okSlug,
		Status:      string(constants.StatusNew),
		Description: ptrString("Software Engineer"),
	}

	mockRepo.EXPECT().GetBySlug(gomock.Any(), badSlug, "").Return(&models.JobWithOwnership{
		Job: &models.Job{ID: 204, Slug: badSlug, Status: string(constants.StatusAnalyzed), Description: ptrString("desc")},
	}, nil)
	mockRepo.EXPECT().GetBySlug(gomock.Any(), okSlug, "").Return(&models.JobWithOwnership{Job: okJob}, nil)
	mockRepo.EXPECT().UpdateStatus(gomock.Any(), okSlug, "queued", (*string)(nil), (*string)(nil), "", "").Return(okJob, nil)
	mockWorkflows.EXPECT().StartLimiterWorkItem(gomock.Any(), "job-analyze-205", models.LimiterJobTypeAnalyze, models.JobAnalyzeWorkflowInput{
		JobDescription: "Software Engineer", JobID: "205", Slug: okSlug,
	}).Return("job-analyze-205", nil)

	svc := &jobService{repo: mockRepo, userJobs: mockUserJobs, workflows: mockWorkflows}
	res, err := svc.QueueAnalysisBulk(context.Background(), []models.QueueAnalysisItem{
		{Slug: badSlug}, {Slug: okSlug},
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, []string{"job-analyze-205"}, res.WorkflowIDs)
	require.Len(t, res.Failed, 1)
	assert.Equal(t, badSlug, res.Failed[0].Slug)
	assert.Contains(t, res.Failed[0].Error, `job "job-analyzed" cannot be queued for analysis with status "analyzed"`)
}

func TestQueueAnalysisBulk_QueuedStaleness(t *testing.T) {
	for _, status := range []constants.JobStatus{constants.StatusQueued, constants.StatusQueuedRetry} {
		t.Run(string(status)+"_recent_rejected", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := repoMocks.NewMockJobs(ctrl)
			mockUserJobs := repoMocks.NewMockUserJobs(ctrl)
			mockWorkflows := repoMocks.NewMockWorkflows(ctrl)

			slug := "job-" + string(status) + "-recent"
			job := &models.Job{
				ID:          301,
				Slug:        slug,
				Status:      string(status),
				Description: ptrString("Software Engineer"),
				UpdatedAt:   time.Now().Add(-10 * time.Minute),
			}

			mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)

			svc := &jobService{repo: mockRepo, userJobs: mockUserJobs, workflows: mockWorkflows}
			res, err := svc.QueueAnalysisBulk(context.Background(), []models.QueueAnalysisItem{{Slug: slug, Force: true}}, nil)

			require.NoError(t, err)
			assert.Empty(t, res.WorkflowIDs)
			require.Len(t, res.Failed, 1)
			assert.Equal(t, slug, res.Failed[0].Slug)
			assert.Contains(t, res.Failed[0].Error, "less than 30 minutes")
		})

		t.Run(string(status)+"_stale_allowed", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := repoMocks.NewMockJobs(ctrl)
			mockUserJobs := repoMocks.NewMockUserJobs(ctrl)
			mockWorkflows := repoMocks.NewMockWorkflows(ctrl)

			slug := "job-" + string(status) + "-stale"
			job := &models.Job{
				ID:          302,
				Slug:        slug,
				Status:      string(status),
				Description: ptrString("Software Engineer"),
				UpdatedAt:   time.Now().Add(-45 * time.Minute),
			}

			mockRepo.EXPECT().GetBySlug(gomock.Any(), slug, "").Return(&models.JobWithOwnership{Job: job}, nil)
			mockRepo.EXPECT().UpdateStatus(gomock.Any(), slug, "queued_retry", (*string)(nil), (*string)(nil), "", "").Return(job, nil)
			mockWorkflows.EXPECT().StartLimiterWorkItem(gomock.Any(), "job-analyze-302", models.LimiterJobTypeAnalyze, models.JobAnalyzeWorkflowInput{
				JobDescription: "Software Engineer",
				JobID:          "302",
				Slug:           slug,
			}).Return("job-analyze-302", nil)

			svc := &jobService{repo: mockRepo, userJobs: mockUserJobs, workflows: mockWorkflows}
			res, err := svc.QueueAnalysisBulk(context.Background(), []models.QueueAnalysisItem{{Slug: slug, Force: true}}, nil)

			require.NoError(t, err)
			assert.Equal(t, []string{"job-analyze-302"}, res.WorkflowIDs)
			assert.Empty(t, res.Failed)
		})
	}
}
