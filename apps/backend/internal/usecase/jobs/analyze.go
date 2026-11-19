package jobs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// GenerateWorkflowID returns the deterministic workflow ID for a job analysis run: job-analyze-{id}.
func GenerateWorkflowID(jobID int64, _ bool) string {
	return fmt.Sprintf("job-analyze-%d", jobID)
}

// QueueAnalysisBulk validates each job listing and starts its analysis workflow in Temporal.
// Allowed job statuses: "new", "failed" (force=true), "queued" and "queued_retry"
// (force=true, updated_at > 30m), and "analyze_pending" (force=true, updated_at >
// 30m, workflow not running). Clerk callers may only queue jobs they own. A failure
// on one item is recorded in Failed and does not stop the remaining items.
func (s *jobService) QueueAnalysisBulk(ctx context.Context, items []models.QueueAnalysisItem, caller *models.Principal) (models.QueueAnalysisBulkResult, error) {
	result := models.QueueAnalysisBulkResult{
		WorkflowIDs: make([]string, 0, len(items)),
	}
	for _, item := range items {
		id, err := s.queueAnalysis(ctx, item.Slug, item.Force, caller)
		if err != nil {
			result.Failed = append(result.Failed, models.QueueAnalysisFailure{Slug: item.Slug, Error: err.Error()})
			continue
		}
		result.WorkflowIDs = append(result.WorkflowIDs, id)
	}
	return result, nil
}

func (s *jobService) queueAnalysis(ctx context.Context, slug string, force bool, caller *models.Principal) (string, error) {
	jobWithOwnership, err := s.repo.GetBySlug(ctx, slug, "")
	if err != nil {
		return "", err
	}
	job := jobWithOwnership.Job

	// Owner-only enforcement for Clerk callers. API key callers can queue any job.
	if caller != nil && caller.ActorType == "" {
		owned, err := s.userJobs.IsOwned(ctx, caller.ID, job.ID)
		if err != nil {
			return "", fmt.Errorf("check job ownership: %w", err)
		}
		if !owned {
			return "", constants.ErrNotFound
		}
	}

	var actorType, actorID string
	if caller != nil {
		actorType = caller.ActorType
		actorID = caller.ActorID
	}

	if job.Description == nil || strings.TrimSpace(*job.Description) == "" {
		return "", fmt.Errorf("job %q %w", job.Slug, constants.ErrNoDescription)
	}

	if s.workflows == nil {
		return "", fmt.Errorf("workflows repository not configured")
	}

	// Pre-condition: status must be new, queued or queued_retry (force=true, updated_at > 30m), failed (force=true), or analyze_pending (force=true, updated_at > 30m, workflow not running).
	// When queueing jobs, update status to queued. When retrying failed or stale jobs, update status to queued_retry before queueing.
	switch constants.JobStatus(job.Status) {
	case constants.StatusNew:
		if _, err := s.repo.UpdateStatus(ctx, models.UpdateJobParams{
			Slug: job.Slug, Status: string(constants.StatusQueued),
			UpdatedByType: actorType, UpdatedByID: actorID,
		}); err != nil {
			return "", fmt.Errorf("update job status to queued: %w", err)
		}
	case constants.StatusQueued, constants.StatusQueuedRetry:
		if !force {
			return "", fmt.Errorf("job %q %w", job.Slug, constants.ErrAlreadyQueued)
		}
		if time.Since(job.UpdatedAt) <= 30*time.Minute {
			return "", fmt.Errorf("%w: job %q has been queued for less than 30 minutes", constants.ErrInvalidTransition, job.Slug)
		}
		if _, err := s.repo.UpdateStatus(ctx, models.UpdateJobParams{
			Slug: job.Slug, Status: string(constants.StatusQueuedRetry),
			UpdatedByType: actorType, UpdatedByID: actorID,
		}); err != nil {
			return "", fmt.Errorf("update job status to queued_retry: %w", err)
		}
	case constants.StatusFailed:
		if !force {
			return "", fmt.Errorf("%w: job %q is failed, force must be true to re-analyze", constants.ErrInvalidTransition, job.Slug)
		}
		if _, err := s.repo.UpdateStatus(ctx, models.UpdateJobParams{
			Slug: job.Slug, Status: string(constants.StatusQueuedRetry),
			UpdatedByType: actorType, UpdatedByID: actorID,
		}); err != nil {
			return "", fmt.Errorf("update job status to queued_retry: %w", err)
		}
	case constants.StatusAnalyzePending:
		if !force {
			return "", fmt.Errorf("%w: job %q is analyze_pending, force must be true to re-analyze", constants.ErrInvalidTransition, job.Slug)
		}
		if time.Since(job.UpdatedAt) <= 30*time.Minute {
			return "", fmt.Errorf("%w: job %q has been pending for less than 30 minutes", constants.ErrInvalidTransition, job.Slug)
		}
		if s.workflows == nil {
			return "", fmt.Errorf("workflows repository not configured")
		}
		running, err := s.workflows.IsWorkflowRunning(ctx, fmt.Sprintf("job-analyze-%d", job.ID))
		if err != nil {
			return "", fmt.Errorf("check workflow status: %w", err)
		}
		if running {
			return "", fmt.Errorf("%w: job %q analysis workflow is currently running", constants.ErrInvalidTransition, job.Slug)
		}
		if _, err := s.repo.UpdateStatus(ctx, models.UpdateJobParams{
			Slug: job.Slug, Status: string(constants.StatusQueuedRetry),
			UpdatedByType: actorType, UpdatedByID: actorID,
		}); err != nil {
			return "", fmt.Errorf("update job status to queued_retry: %w", err)
		}
	default:
		return "", fmt.Errorf("%w: job %q cannot be queued for analysis with status %q", constants.ErrInvalidTransition, job.Slug, job.Status)
	}

	workflowID := GenerateWorkflowID(job.ID, force)

	if s.workflows == nil {
		return "", fmt.Errorf("workflows repository not configured")
	}

	input := models.JobAnalyzeWorkflowInput{
		JobDescription: *job.Description,
		JobID:          strconv.FormatInt(job.ID, 10),
		Slug:           job.Slug,
	}

	return s.workflows.StartLimiterWorkItem(ctx, workflowID, models.LimiterJobTypeAnalyze, input)
}
