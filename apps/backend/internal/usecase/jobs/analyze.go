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

// GenerateWorkflowID returns the deterministic workflow ID for a job analysis run.
// Default (force=false): job-analyze-{id}.
// Force (force=true): job-analyze-{id}-{epoch_ms}.
func GenerateWorkflowID(jobID int64, force bool) string {
	if !force {
		return fmt.Sprintf("job-analyze-%d", jobID)
	}
	return fmt.Sprintf("job-analyze-%d-%d", jobID, time.Now().UnixMilli())
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

	// Pre-condition: status must be new or failed. If failed, force must be true.
	switch constants.JobStatus(job.Status) {
	case constants.StatusNew:
		// Allowed.
	case constants.StatusFailed:
		if !force {
			return "", fmt.Errorf("%w: job %q is failed, force must be true to re-analyze", constants.ErrInvalidTransition, job.Slug)
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
