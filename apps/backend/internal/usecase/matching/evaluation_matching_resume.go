package matching

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// UpdateEvaluationResumeRequest holds the payload sent by the worker to update a resume.
type UpdateEvaluationResumeRequest struct {
	ResumeStatus         string
	ResumeFailureMessage *string
	ResumePDFKey         *string
	ResumeJSONKey        *string
}

func (s *matchingService) UpdateEvaluationResume(
	ctx context.Context,
	id uuid.UUID,
	req UpdateEvaluationResumeRequest,
) (*models.MatchingEvaluation, error) {
	if _, ok := map[string]bool{
		string(constants.ResumeStatusQueued):      true,
		string(constants.ResumeStatusQueuedRetry): true,
		string(constants.ResumeStatusPending):     true,
		string(constants.ResumeStatusSuccess):     true,
		string(constants.ResumeStatusFailed):      true,
	}[req.ResumeStatus]; !ok {
		return nil, fmt.Errorf("%w: invalid resume_status %s", constants.ErrInvalidRequest, req.ResumeStatus)
	}
	return s.evalRepo.UpdateResume(ctx, id, models.UpdateMatchingEvaluationResumeParams{
		ResumeStatus:         req.ResumeStatus,
		ResumeFailureMessage: req.ResumeFailureMessage,
		ResumePDFKey:         req.ResumePDFKey,
		ResumeJSONKey:        req.ResumeJSONKey,
	})
}

func (s *matchingService) triggerResume(
	ctx context.Context,
	id uuid.UUID,
	force bool,
) (*models.MatchingEvaluation, error) {
	eval, err := s.evalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if eval.EvaluationStatus != string(constants.MatchingStatusSuccess) {
		return nil, fmt.Errorf("%w: evaluation must be successful", constants.ErrInvalidRequest)
	}
	if eval.ReportStatus == nil || *eval.ReportStatus != string(constants.ReportStatusSuccess) {
		return nil, fmt.Errorf("%w: report must be successful", constants.ErrInvalidRequest)
	}
	if eval.ResumeStatus != nil && *eval.ResumeStatus == string(constants.ResumeStatusSuccess) {
		return nil, fmt.Errorf("%w: resume already successfully generated; regeneration not allowed", constants.ErrInvalidRequest)
	}
	if eval.ResumeStatus != nil && (*eval.ResumeStatus == string(constants.ResumeStatusQueued) || *eval.ResumeStatus == string(constants.ResumeStatusQueuedRetry) || *eval.ResumeStatus == string(constants.ResumeStatusPending)) && !force {
		return nil, fmt.Errorf("%w: resume generation already in progress", constants.ErrInvalidRequest)
	}
	if eval.ResumeStatus != nil && (*eval.ResumeStatus == string(constants.ResumeStatusQueued) || *eval.ResumeStatus == string(constants.ResumeStatusQueuedRetry)) {
		if err := s.requireStaleNotRunning(ctx, eval.UpdatedAt, fmt.Sprintf("resume-gen-%s", id.String()), "resume generation"); err != nil {
			return nil, err
		}
	}

	userJob, err := s.userJobsRepo.GetByID(ctx, eval.UserJobID)
	if err != nil {
		return nil, fmt.Errorf("get user job: %w", err)
	}
	user, err := s.usersRepo.FindByID(ctx, userJob.UserID.String())
	if err != nil {
		return nil, fmt.Errorf("get resume user: %w", err)
	}
	if user.OnboardingCompletedAt == nil {
		return nil, fmt.Errorf("%w: onboarding is incomplete", constants.ErrInvalidRequest)
	}

	queueStatus := string(constants.ResumeStatusQueued)
	if eval.ResumeStatus != nil {
		queueStatus = string(constants.ResumeStatusQueuedRetry)
	}
	updated, err := s.evalRepo.UpdateResume(ctx, id, models.UpdateMatchingEvaluationResumeParams{
		ResumeStatus: queueStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("update resume status to %s: %w", queueStatus, err)
	}
	eval = updated

	workflowID := fmt.Sprintf("resume-gen-%s", id.String())
	input := map[string]interface{}{"evaluation_id": id.String()}
	if _, err := s.workflowsRepo.StartLimiterWorkItem(ctx, workflowID, models.LimiterJobTypeResume, input); err != nil {
		return nil, fmt.Errorf("start resume workflow: %w", err)
	}
	return eval, nil
}

func (s *matchingService) GetResume(
	ctx context.Context,
	id uuid.UUID,
	caller *models.Principal,
) (*models.MatchingEvaluation, error) {
	if caller == nil {
		return nil, constants.ErrUnauthorized
	}
	eval, err := s.evalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if caller.ActorType == "" {
		userJob, err := s.userJobsRepo.GetByID(ctx, eval.UserJobID)
		if err != nil {
			return nil, fmt.Errorf("get user job ownership: %w", err)
		}
		if userJob.UserID.String() != caller.ID {
			return nil, constants.ErrForbidden
		}
	}
	return eval, nil
}

func (s *matchingService) DownloadResume(
	ctx context.Context,
	id uuid.UUID,
	caller *models.Principal,
) (*models.DownloadResumeResponse, error) {
	if caller == nil {
		return nil, constants.ErrUnauthorized
	}
	eval, err := s.evalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if eval.ResumeStatus == nil || *eval.ResumeStatus != string(constants.ResumeStatusSuccess) {
		return nil, fmt.Errorf("%w: resume is not available", constants.ErrInvalidRequest)
	}
	if eval.ResumePDFKey == nil || strings.TrimSpace(*eval.ResumePDFKey) == "" {
		return nil, fmt.Errorf("%w: resume pdf key is missing", constants.ErrInvalidRequest)
	}

	userJob, err := s.userJobsRepo.GetByID(ctx, eval.UserJobID)
	if err != nil {
		return nil, fmt.Errorf("get user job: %w", err)
	}
	if caller.ActorType == "" && userJob.UserID.String() != caller.ID {
		return nil, constants.ErrForbidden
	}

	user, err := s.usersRepo.FindByID(ctx, userJob.UserID.String())
	name := ""
	if err == nil && user != nil && user.FullName != nil {
		name = strings.TrimSpace(*user.FullName)
	}

	// The job profile is the analysis result and has no title, so read the
	// position from the job listing itself.
	withJob, err := s.evalRepo.GetLatestByUserJobID(ctx, userJob.UserID, eval.UserJobID)
	if err != nil {
		return nil, fmt.Errorf("get job for resume download: %w", err)
	}
	jobTitle := ""
	if withJob != nil && withJob.Job != nil {
		jobTitle = strings.TrimSpace(withJob.Job.Title)
	}

	filename := resumeFileName(name, jobTitle)

	if s.storageRepo == nil {
		return nil, errors.New("storage repository is not configured")
	}
	expiry := 15 * time.Minute
	downloadURL, err := s.storageRepo.GetPresignedDownloadURL(ctx, *eval.ResumePDFKey, filename, expiry)
	if err != nil {
		return nil, fmt.Errorf("generate presigned download url: %w", err)
	}

	return &models.DownloadResumeResponse{
		DownloadURL: downloadURL,
		ExpiresAt:   time.Now().Add(expiry),
		Filename:    filename,
	}, nil
}

// resumeFileName builds the resume download file name as
// Name_Resume_Job-Title.pdf, falling back gracefully when the candidate name
// or job title is unavailable. Spaces in the name become underscores and
// spaces in the job title become hyphens. Blank names/titles are omitted.
func resumeFileName(name, jobTitle string) string {
	cleanName := strings.ReplaceAll(strings.TrimSpace(name), " ", "_")
	cleanTitle := strings.ReplaceAll(strings.TrimSpace(jobTitle), " ", "-")

	parts := []string{}
	if cleanName != "" {
		parts = append(parts, cleanName)
	}
	parts = append(parts, "Resume")
	if cleanTitle != "" {
		parts = append(parts, cleanTitle)
	}

	filename := strings.Join(parts, "_")
	if filename == "" || filename == "Resume" {
		return "Resume.pdf"
	}
	return filename + ".pdf"
}

func (s *matchingService) BulkQueueGenerateResume(
	ctx context.Context,
	items []models.BulkGenerateResumeItem,
	caller *models.Principal,
) ([]*models.MatchingEvaluationSummary, error) {
	if caller == nil {
		return nil, constants.ErrUnauthorized
	}
	summaries := make([]*models.MatchingEvaluationSummary, 0, len(items))
	for _, item := range items {
		// Clerk users may only retry resume generation on evaluations they own.
		if caller.ActorType == "" {
			owned, err := s.isEvaluationOwnedBy(ctx, item.ID, caller.ID)
			if err != nil {
				message := err.Error()
				summaries = append(summaries, &models.MatchingEvaluationSummary{ID: &item.ID, Error: &message})
				continue
			}
			if !owned {
				message := "not authorized to generate resume for this evaluation"
				summaries = append(summaries, &models.MatchingEvaluationSummary{ID: &item.ID, Error: &message})
				continue
			}
		}

		eval, err := s.triggerResume(ctx, item.ID, item.Force)
		if err != nil {
			message := err.Error()
			summaries = append(summaries, &models.MatchingEvaluationSummary{ID: &item.ID, Error: &message})
			continue
		}
		summaries = append(summaries, &models.MatchingEvaluationSummary{
			ID: &eval.ID, UserJobID: &eval.UserJobID, ResumeStatus: eval.ResumeStatus, CreatedAt: &eval.CreatedAt,
		})
	}
	return summaries, nil
}

// isEvaluationOwnedBy reports whether the matching evaluation belongs to the
// given internal user via its user_job link.
func (s *matchingService) isEvaluationOwnedBy(ctx context.Context, evalID uuid.UUID, userID string) (bool, error) {
	eval, err := s.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return false, fmt.Errorf("get evaluation: %w", err)
	}
	userJob, err := s.userJobsRepo.GetByID(ctx, eval.UserJobID)
	if err != nil {
		return false, fmt.Errorf("get user job: %w", err)
	}
	return userJob.UserID.String() == userID, nil
}
