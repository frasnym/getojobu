package matching

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// QueueCoverLetterRequest holds the caller's cover-letter generation request.
type QueueCoverLetterRequest struct {
	Variant string
	Force   bool
}

// UpdateEvaluationCoverLetterRequest holds the worker's cover-letter status update.
type UpdateEvaluationCoverLetterRequest struct {
	CoverLetterStatus         string
	CoverLetterContent        *models.CoverLetterContent
	CoverLetterFailureMessage *string
}

// CoverLetterGenerationFailedMessage is the only failure text exposed to users.
// Raw exceptions may contain prompts or CV contents and must never be returned.
const CoverLetterGenerationFailedMessage = "Cover letter generation failed"

const maxCoverLetterFailureLines = 10

// appendFailureLine appends a timestamped failure line and keeps only the most
// recent lines. The newest line is always last.
func appendFailureLine(existing, message string, now time.Time) string {
	lines := make([]string, 0, maxCoverLetterFailureLines+1)
	for _, line := range strings.Split(existing, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	lines = append(lines, now.UTC().Format(time.RFC3339)+" — "+message)
	if len(lines) > maxCoverLetterFailureLines {
		lines = lines[len(lines)-maxCoverLetterFailureLines:]
	}
	return strings.Join(lines, "\n")
}

// CoverLetterFileName builds the on-demand download file name as
// Name_Cover Letter_Position.pdf. Both optional parts and their underscores
// are omitted when missing. Path separators and other filename-unsafe
// characters are stripped.
func CoverLetterFileName(name, jobTitle string) string {
	cleanName := utils.SanitizeFileNamePart(name)
	cleanTitle := utils.SanitizeFileNamePart(jobTitle)

	parts := []string{}
	if cleanName != "" {
		parts = append(parts, cleanName)
	}
	parts = append(parts, "Cover Letter")
	if cleanTitle != "" {
		parts = append(parts, cleanTitle)
	}
	return strings.Join(parts, "_") + ".pdf"
}

// UpdateEvaluationCoverLetter persists a worker status update. The previous
// letter is preserved until a new one succeeds, and failure history is
// append-only with a safe, fixed message.
func (s *matchingService) UpdateEvaluationCoverLetter(
	ctx context.Context,
	id uuid.UUID,
	req UpdateEvaluationCoverLetterRequest,
) (*models.MatchingEvaluation, error) {
	if !constants.IsValidCoverLetterStatus(req.CoverLetterStatus) {
		return nil, fmt.Errorf("%w: invalid cover_letter_status %s", constants.ErrInvalidRequest, req.CoverLetterStatus)
	}

	existing, err := s.evalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	params := models.UpdateMatchingEvaluationCoverLetterParams{
		CoverLetterStatus:         req.CoverLetterStatus,
		CoverLetterContent:        existing.CoverLetterContent,
		CoverLetterFailureMessage: existing.CoverLetterFailureMessage,
	}

	if req.CoverLetterStatus == "failed" {
		params.CoverLetterFailureMessage = utils.PtrString(appendFailureLine(
			utils.PtrStringOrEmpty(existing.CoverLetterFailureMessage),
			CoverLetterGenerationFailedMessage,
			time.Now(),
		))
	} else if req.CoverLetterContent != nil {
		contentBytes, err := json.Marshal(req.CoverLetterContent)
		if err != nil {
			return nil, fmt.Errorf("marshal cover letter content: %w", err)
		}
		params.CoverLetterContent = contentBytes
	}

	return s.evalRepo.UpdateCoverLetter(ctx, id, params)
}

// QueueCoverLetter validates preconditions, reserves the queued state and
// starts the cover-letter generation workflow item.
//
// The status is written before the workflow is enqueued. If enqueueing fails,
// the evaluation can be left in queued/queued_retry with no live work item;
// that stuck state is recoverable with a forced request, which uses a fresh
// workflow ID (see below) and therefore bypasses the limiter's per-item
// deduplication.
func (s *matchingService) QueueCoverLetter(
	ctx context.Context,
	id uuid.UUID,
	req QueueCoverLetterRequest,
	caller *models.Principal,
) (*models.MatchingEvaluationSummary, error) {
	if caller == nil {
		return nil, constants.ErrUnauthorized
	}
	if !constants.IsValidCoverLetterVariant(req.Variant) {
		return nil, fmt.Errorf("%w: invalid cover letter variant %s", constants.ErrInvalidRequest, req.Variant)
	}

	eval, err := s.evalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if eval.EvaluationStatus != string(constants.MatchingStatusSuccess) {
		return nil, fmt.Errorf("%w: evaluation must be successful", constants.ErrInvalidRequest)
	}

	// Capture a stable nonce from the row loaded before the status write. Forced
	// runs use it so a retry can start a fresh limiter item when the previous
	// item is still active or queued (the limiter deduplicates by item_id).
	// Two rapid force clicks on this same loaded row share an ID, while a later
	// force after a status change (and thus a new UpdatedAt) gets a new one.
	forceNonce := eval.UpdatedAt.UnixNano()

	currentStatus := ""
	if eval.CoverLetterStatus != nil {
		currentStatus = *eval.CoverLetterStatus
	}
	switch currentStatus {
	case "success":
		if !req.Force {
			return nil, fmt.Errorf("%w: cover letter already successfully generated; regeneration not allowed", constants.ErrInvalidRequest)
		}
	case "queued", "queued_retry", "pending":
		if !req.Force {
			return nil, fmt.Errorf("%w: cover letter generation already in progress", constants.ErrInvalidRequest)
		}
	}

	userJob, err := s.userJobsRepo.GetByID(ctx, eval.UserJobID)
	if err != nil {
		return nil, fmt.Errorf("get user job: %w", err)
	}
	if caller.ActorType == "" && userJob.UserID.String() != caller.ID {
		return nil, constants.ErrForbidden
	}

	user, err := s.usersRepo.FindByID(ctx, userJob.UserID.String())
	if err != nil {
		return nil, fmt.Errorf("get cover letter user: %w", err)
	}
	if user.OnboardingCompletedAt == nil {
		return nil, fmt.Errorf("%w: onboarding is incomplete", constants.ErrInvalidRequest)
	}

	input := buildCoverLetterWorkflowInput(eval, userJob, user, req.Variant)

	queueStatus := "queued"
	if currentStatus != "" {
		queueStatus = "queued_retry"
	}
	// The repository assigns these columns directly, so carry the existing
	// values forward: a queued regeneration keeps the last good letter and the
	// accumulated failure history visible until the worker writes new data.
	updated, err := s.evalRepo.UpdateCoverLetter(ctx, id, models.UpdateMatchingEvaluationCoverLetterParams{
		CoverLetterStatus:         queueStatus,
		CoverLetterContent:        eval.CoverLetterContent,
		CoverLetterFailureMessage: eval.CoverLetterFailureMessage,
	})
	if err != nil {
		return nil, fmt.Errorf("update cover letter status to %s: %w", queueStatus, err)
	}
	eval = updated

	// Non-force requests keep the deterministic ID so double-clicks collapse
	// into a single limiter item. Forced requests add the nonce so a stuck or
	// still-active item does not absorb the retry.
	workflowID := fmt.Sprintf("cover-letter-gen-%s", id.String())
	if req.Force {
		workflowID = fmt.Sprintf("cover-letter-gen-%s-%d", id.String(), forceNonce)
	}
	if _, err := s.workflowsRepo.StartLimiterWorkItem(ctx, workflowID, models.LimiterJobTypeCoverLetter, input); err != nil {
		return nil, fmt.Errorf("start cover letter workflow: %w", err)
	}

	return &models.MatchingEvaluationSummary{
		ID:                &eval.ID,
		UserJobID:         &eval.UserJobID,
		EvaluationStatus:  eval.EvaluationStatus,
		CoverLetterStatus: eval.CoverLetterStatus,
		CreatedAt:         &eval.CreatedAt,
	}, nil
}

// buildCoverLetterWorkflowInput mirrors the resume generation payload and adds
// the chosen variant. All I/O stays in the usecase; the workflow receives a
// plain, self-contained map.
func buildCoverLetterWorkflowInput(
	eval *models.MatchingEvaluation,
	userJob *models.UserJob,
	user *models.User,
	variant string,
) map[string]interface{} {
	var candidate, job, evidence map[string]interface{}
	_ = json.Unmarshal(eval.CandidateProfileData, &candidate)
	_ = json.Unmarshal(eval.JobProfileData, &job)
	_ = json.Unmarshal(eval.EvaluationData, &evidence)
	if candidate == nil {
		candidate = map[string]interface{}{}
	}
	if job == nil {
		job = map[string]interface{}{}
	}
	if evidence == nil {
		evidence = map[string]interface{}{}
	}

	var roles []map[string]interface{}
	if rawExps, ok := candidate["experience"].([]interface{}); ok {
		for idx, rawExp := range rawExps {
			if expMap, ok := rawExp.(map[string]interface{}); ok {
				roles = append(roles, map[string]interface{}{
					"id":      fmt.Sprintf("exp-%d", idx+1),
					"company": expMap["company"],
					"role":    expMap["role"],
					"period":  expMap["period"],
				})
			}
		}
	}

	var contactParts []string
	for _, value := range []*string{user.FullName, user.Phone, user.Email, user.LinkedInURL, user.GitHubURL} {
		if value != nil && strings.TrimSpace(*value) != "" {
			contactParts = append(contactParts, *value)
		}
	}

	education := ""
	if value, ok := candidate["education"].(string); ok {
		education = value
	}

	facts := map[string]interface{}{
		"name":      utils.PtrStringOrEmpty(user.FullName),
		"headline":  utils.PtrStringOrEmpty(user.Headline),
		"phone":     utils.PtrStringOrEmpty(user.Phone),
		"email":     utils.PtrStringOrEmpty(user.Email),
		"linkedin":  utils.PtrStringOrEmpty(user.LinkedInURL),
		"github":    utils.PtrStringOrEmpty(user.GitHubURL),
		"portfolio": utils.PtrStringOrEmpty(user.PortfolioURL),
		"education": education,
		"contact":   strings.Join(contactParts, " | "),
		"roles":     roles,
	}

	return map[string]interface{}{
		"evaluation_id":        eval.ID.String(),
		"job_id":               userJob.JobID,
		"user_id":              userJob.UserID.String(),
		"candidate_profile":    candidate,
		"facts":                facts,
		"job_profile":          job,
		"evaluation_results":   evidence,
		"overall_score":        utils.PtrFloatOrZero(eval.OverallScore),
		"eligibility_status":   utils.PtrStringOrEmpty(eval.EligibilityStatus),
		"recommendation_label": utils.PtrStringOrEmpty(eval.RecommendationLabel),
		"variant":              variant,
	}
}
