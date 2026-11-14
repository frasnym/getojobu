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
// Allowed job statuses: "new" (force=false or true) and "failed" (force=true only).
// Clerk callers may only queue jobs they own.
func (s *jobService) QueueAnalysisBulk(ctx context.Context, items []models.QueueAnalysisItem, caller *models.Principal) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}

	workflowIDs := make([]string, 0, len(items))
	for _, item := range items {
		id, err := s.queueAnalysis(ctx, item.Slug, item.Force, caller)
		if err != nil {
			return nil, err
		}
		workflowIDs = append(workflowIDs, id)
	}
	return workflowIDs, nil
}

func (s *jobService) queueAnalysis(ctx context.Context, slug string, force bool, caller *models.Principal) (string, error) {
	job, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return "", err
	}

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

	// Pre-condition: status must be new, new-retry, failed (force=true), or analyze_pending (force=true, updated_at > 30m, workflow not running).
	// When retrying failed or stale analyze_pending jobs, update status to new-retry before queueing.
	switch constants.JobStatus(job.Status) {
	case constants.StatusNew, constants.StatusNewRetry:
		// Ready to queue.
	case constants.StatusFailed:
		if !force {
			return "", fmt.Errorf("%w: job %q is failed, force must be true to re-analyze", constants.ErrInvalidTransition, job.Slug)
		}
		if _, err := s.repo.UpdateStatus(ctx, job.Slug, string(constants.StatusNewRetry), nil, nil, actorType, actorID); err != nil {
			return "", fmt.Errorf("update job status to new-retry: %w", err)
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
		if _, err := s.repo.UpdateStatus(ctx, job.Slug, string(constants.StatusNewRetry), nil, nil, actorType, actorID); err != nil {
			return "", fmt.Errorf("update job status to new-retry: %w", err)
		}
	default:
		return "", fmt.Errorf("%w: job %q cannot be queued for analysis with status %q", constants.ErrInvalidTransition, job.Slug, job.Status)
	}

	if job.Description == nil || strings.TrimSpace(*job.Description) == "" {
		return "", fmt.Errorf("%w: job %q has no description to analyze", constants.ErrInvalidTransition, job.Slug)
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
