// Package retry re-queues failed Temporal workflows discovered in the
// application database. It reuses the same usecases the public API calls, so a
// swept failure follows the same state transitions as a manual force retry.
package retry

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repo "github.com/frasnym/getojobu/apps/backend/internal/repository"
	ucJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs"
	ucMatching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

const (
	// MaxFailedJobs caps job analysis retries per sweep.
	MaxFailedJobs = 20
	// MaxFailedMatchingEvaluations caps evaluation rows read per sweep.
	MaxFailedMatchingEvaluations = 50
)

// Retry re-queues failed workflow states.
type Retry interface {
	FailedJobs(ctx context.Context, caller *models.Principal) (models.RetryJobsResult, error)
	FailedMatchingEvaluations(ctx context.Context, caller *models.Principal) (models.RetryMatchingEvaluationsResult, error)
}

type retryService struct {
	repo     repo.Retry
	jobs     ucJobs.Job
	matching ucMatching.Matching
}

// NewService builds a retry service from the repository and usecase contracts.
func NewService(r *repository.Contract, jobsUC ucJobs.Job, matchingUC ucMatching.Matching) Retry {
	return &retryService{
		repo:     r.RetryRepository,
		jobs:     jobsUC,
		matching: matchingUC,
	}
}

func (s *retryService) FailedJobs(ctx context.Context, caller *models.Principal) (models.RetryJobsResult, error) {
	result := models.RetryJobsResult{}

	slugs, err := s.repo.ListFailedJobs(ctx, MaxFailedJobs)
	if err != nil {
		return result, fmt.Errorf("list failed jobs: %w", err)
	}

	for _, slug := range slugs {
		res, err := s.jobs.QueueAnalysisBulk(ctx, []models.QueueAnalysisItem{{Slug: slug, Force: true}}, caller)
		if err != nil {
			logging.Warn(ctx, "retry failed job analyze", "slug", slug, "error", err)
			continue
		}
		result.Requeued += len(res.WorkflowIDs)
	}

	return result, nil
}

func (s *retryService) FailedMatchingEvaluations(ctx context.Context, caller *models.Principal) (models.RetryMatchingEvaluationsResult, error) {
	result := models.RetryMatchingEvaluationsResult{}

	evals, err := s.repo.ListFailedMatchingEvaluations(ctx, MaxFailedMatchingEvaluations)
	if err != nil {
		return result, fmt.Errorf("list failed matching evaluations: %w", err)
	}

	var triggerItems []models.TriggerEvaluationItem
	var reports, resumes []uuid.UUID
	var coverLetters []models.BulkGenerateCoverLetterItem
	for _, eval := range evals {
		if eval.EvaluationStatus == string(constants.MatchingStatusFailed) {
			triggerItems = append(triggerItems, models.TriggerEvaluationItem{ID: eval.ID})
		}
		if isFailed(eval.ReportStatus) {
			reports = append(reports, eval.ID)
		}
		if isFailed(eval.ResumeStatus) {
			resumes = append(resumes, eval.ID)
		}
		if isFailed(eval.CoverLetterStatus) {
			coverLetters = append(coverLetters, models.BulkGenerateCoverLetterItem{
				ID:      eval.ID,
				Variant: "short",
				Force:   true,
			})
		}
	}

	if len(triggerItems) > 0 {
		summaries, err := s.matching.TriggerEvaluations(ctx, triggerItems, caller)
		if err != nil {
			logging.Warn(ctx, "retry failed evaluations", "error", err)
		} else {
			result.Evaluations = queuedCount(summaries)
		}
	}

	if len(reports) > 0 {
		batch := make([]models.BulkGenerateReportItem, 0, len(reports))
		for _, id := range reports {
			batch = append(batch, models.BulkGenerateReportItem{ID: id, Force: true})
		}
		summaries, err := s.matching.BulkQueueGenerateReport(ctx, batch, caller)
		if err != nil {
			logging.Warn(ctx, "retry failed reports", "error", err)
		} else {
			result.Reports = queuedCount(summaries)
		}
	}

	if len(resumes) > 0 {
		batch := make([]models.BulkGenerateResumeItem, 0, len(resumes))
		for _, id := range resumes {
			batch = append(batch, models.BulkGenerateResumeItem{ID: id, Force: true})
		}
		summaries, err := s.matching.BulkQueueGenerateResume(ctx, batch, caller)
		if err != nil {
			logging.Warn(ctx, "retry failed resumes", "error", err)
		} else {
			result.Resumes = queuedCount(summaries)
		}
	}

	if len(coverLetters) > 0 {
		summaries, err := s.matching.BulkQueueCoverLetter(ctx, coverLetters, caller)
		if err != nil {
			logging.Warn(ctx, "retry failed cover letters", "error", err)
		} else {
			result.CoverLetters = queuedCount(summaries)
		}
	}

	return result, nil
}

func isFailed(status *string) bool {
	return status != nil && *status == "failed"
}

func queuedCount(summaries []*models.MatchingEvaluationSummary) int {
	count := 0
	for _, summary := range summaries {
		if summary != nil && summary.Error == nil {
			count++
		}
	}
	return count
}
