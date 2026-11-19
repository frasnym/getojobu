package retry_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	contractRepo "github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repoMocks "github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	jobsMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs/mocks"
	matchingMocks "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching/mocks"
	ucRetry "github.com/frasnym/getojobu/apps/backend/internal/usecase/retry"
)

type deps struct {
	svc      ucRetry.Retry
	repo     *repoMocks.MockRetry
	jobs     *jobsMocks.MockJob
	matching *matchingMocks.MockMatching
}

func newDeps(t *testing.T) deps {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := deps{
		repo:     repoMocks.NewMockRetry(ctrl),
		jobs:     jobsMocks.NewMockJob(ctrl),
		matching: matchingMocks.NewMockMatching(ctrl),
	}
	d.svc = ucRetry.NewService(&contractRepo.Contract{RetryRepository: d.repo}, d.jobs, d.matching)
	return d
}

var caller = &models.Principal{ActorType: "worker", ActorID: "worker-1"}

func strPtr(s string) *string { return &s }

func TestFailedJobsRequeuesEachWithForce(t *testing.T) {
	d := newDeps(t)
	d.repo.EXPECT().ListFailedJobs(gomock.Any(), ucRetry.MaxFailedJobs).Return([]string{"job-a", "job-b"}, nil)
	for _, slug := range []string{"job-a", "job-b"} {
		d.jobs.EXPECT().QueueAnalysisBulk(gomock.Any(), []models.QueueAnalysisItem{{Slug: slug, Force: true}}, caller).
			Return(models.QueueAnalysisBulkResult{WorkflowIDs: []string{"job-analyze-1"}}, nil)
	}

	result, err := d.svc.FailedJobs(context.Background(), caller)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Requeued)
}

func TestFailedJobsContinuesWhenOneFails(t *testing.T) {
	d := newDeps(t)
	d.repo.EXPECT().ListFailedJobs(gomock.Any(), ucRetry.MaxFailedJobs).Return([]string{"job-a", "job-b"}, nil)
	d.jobs.EXPECT().QueueAnalysisBulk(gomock.Any(), gomock.Any(), caller).
		Return(models.QueueAnalysisBulkResult{Failed: []models.QueueAnalysisFailure{{Slug: "job-a", Error: "no description"}}}, nil)
	d.jobs.EXPECT().QueueAnalysisBulk(gomock.Any(), gomock.Any(), caller).
		Return(models.QueueAnalysisBulkResult{WorkflowIDs: []string{"job-analyze-2"}}, nil)

	result, err := d.svc.FailedJobs(context.Background(), caller)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Requeued)
}

func TestFailedMatchingEvaluationsRequeuesEveryFailedField(t *testing.T) {
	d := newDeps(t)
	id := uuid.New()
	eval := models.FailedMatchingEvaluation{
		ID:                id,
		Slug:              "job-a",
		EvaluationStatus:  "failed",
		ReportStatus:      strPtr("failed"),
		ResumeStatus:      strPtr("failed"),
		CoverLetterStatus: strPtr("failed"),
	}
	d.repo.EXPECT().ListFailedMatchingEvaluations(gomock.Any(), ucRetry.MaxFailedMatchingEvaluations).
		Return([]models.FailedMatchingEvaluation{eval}, nil)
	d.matching.EXPECT().TriggerEvaluations(gomock.Any(), []models.TriggerEvaluationItem{{ID: id}}, caller).
		Return([]*models.MatchingEvaluationSummary{{}}, nil)
	d.matching.EXPECT().BulkQueueGenerateReport(gomock.Any(), []models.BulkGenerateReportItem{{ID: id, Force: true}}, caller).
		Return([]*models.MatchingEvaluationSummary{{}}, nil)
	d.matching.EXPECT().BulkQueueGenerateResume(gomock.Any(), []models.BulkGenerateResumeItem{{ID: id, Force: true}}, caller).
		Return([]*models.MatchingEvaluationSummary{{}}, nil)
	d.matching.EXPECT().BulkQueueCoverLetter(gomock.Any(), []models.BulkGenerateCoverLetterItem{{ID: id, Variant: "short", Force: true}}, caller).
		Return([]*models.MatchingEvaluationSummary{{}}, nil)

	result, err := d.svc.FailedMatchingEvaluations(context.Background(), caller)
	require.NoError(t, err)
	assert.Equal(t, models.RetryMatchingEvaluationsResult{
		Evaluations: 1, Reports: 1, Resumes: 1, CoverLetters: 1,
	}, result)
}

func TestFailedMatchingEvaluationsTriggersEachFailedEvaluationWithoutSlugDedupe(t *testing.T) {
	d := newDeps(t)
	id1, id2 := uuid.New(), uuid.New()
	evals := []models.FailedMatchingEvaluation{
		{ID: id1, Slug: "job-a", EvaluationStatus: "failed", ReportStatus: strPtr("failed")},
		{ID: id2, Slug: "job-a", EvaluationStatus: "failed", ResumeStatus: strPtr("success")},
	}
	d.repo.EXPECT().ListFailedMatchingEvaluations(gomock.Any(), ucRetry.MaxFailedMatchingEvaluations).Return(evals, nil)
	d.matching.EXPECT().TriggerEvaluations(gomock.Any(), []models.TriggerEvaluationItem{
		{ID: id1},
		{ID: id2},
	}, caller).Return([]*models.MatchingEvaluationSummary{{}, {}}, nil)
	d.matching.EXPECT().BulkQueueGenerateReport(gomock.Any(), []models.BulkGenerateReportItem{{ID: id1, Force: true}}, caller).
		Return([]*models.MatchingEvaluationSummary{{}}, nil)

	result, err := d.svc.FailedMatchingEvaluations(context.Background(), caller)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Evaluations)
	assert.Equal(t, 1, result.Reports)
	assert.Equal(t, 0, result.Resumes)
	assert.Equal(t, 0, result.CoverLetters)
}

func TestFailedMatchingEvaluationsWithNoFailuresMakesNoCalls(t *testing.T) {
	d := newDeps(t)
	d.repo.EXPECT().ListFailedMatchingEvaluations(gomock.Any(), ucRetry.MaxFailedMatchingEvaluations).
		Return([]models.FailedMatchingEvaluation{}, nil)

	result, err := d.svc.FailedMatchingEvaluations(context.Background(), caller)
	require.NoError(t, err)
	assert.Equal(t, models.RetryMatchingEvaluationsResult{}, result)
}
