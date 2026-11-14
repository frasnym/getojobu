package matching

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
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
		string(constants.ResumeStatusPending):      true,
		string(constants.ResumeStatusPendingRetry): true,
		string(constants.ResumeStatusSuccess):      true,
		string(constants.ResumeStatusFailed):       true,
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
	if eval.RecommendationLabel == nil ||
		(*eval.RecommendationLabel != string(constants.RecommendationStrongMatch) &&
			*eval.RecommendationLabel != string(constants.RecommendationWorthApplying)) {
		return nil, fmt.Errorf("%w: recommendation is not eligible for resume generation", constants.ErrInvalidRequest)
	}
	if eval.ResumeStatus != nil && *eval.ResumeStatus == string(constants.ResumeStatusSuccess) {
		return nil, fmt.Errorf("%w: resume already successfully generated; regeneration not allowed", constants.ErrInvalidRequest)
	}
	if eval.ResumeStatus != nil && (*eval.ResumeStatus == string(constants.ResumeStatusPending) || *eval.ResumeStatus == string(constants.ResumeStatusPendingRetry)) && !force {
		return nil, fmt.Errorf("%w: resume generation already in progress", constants.ErrInvalidRequest)
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

	var candidate, job, evidence map[string]interface{}
	_ = json.Unmarshal(eval.CandidateProfileData, &candidate)
	_ = json.Unmarshal(eval.JobProfileData, &job)
	_ = json.Unmarshal(eval.EvaluationData, &evidence)

	// Assemble education from user_educations table
	var educationLine string
	var eduRows []*models.UserEducation
	if s.eduRepo != nil {
		edus, err := s.eduRepo.ListByUser(ctx, user.ID)
		if err == nil && len(edus) > 0 {
			eduRows = edus
			top := edus[0]
			var yearStr string
			if top.StartYear != nil && top.EndYear != nil {
				yearStr = fmt.Sprintf("%d-%d", *top.StartYear, *top.EndYear)
			} else if top.EndYear != nil {
				yearStr = fmt.Sprintf("%d", *top.EndYear)
			} else if top.StartYear != nil {
				yearStr = fmt.Sprintf("%d", *top.StartYear)
			}
			parts := []string{fmt.Sprintf("%s in %s, %s", top.Degree, top.FieldOfStudy, top.Institution)}
			if yearStr != "" {
				parts = append(parts, yearStr)
			}
			if top.GPA != nil && *top.GPA != "" {
				parts = append(parts, fmt.Sprintf("GPA: %s", *top.GPA))
			}
			educationLine = strings.Join(parts, " | ")
		}
	}

	// Assemble roles with stable IDs from user_experiences table
	var roles []map[string]interface{}
	if s.expRepo != nil {
		exps, err := s.expRepo.ListByUser(ctx, user.ID)
		if err == nil {
			for _, exp := range exps {
				period := ""
				if !exp.StartedAt.IsZero() {
					start := exp.StartedAt.Format("Jan 2006")
					end := "Current"
					if !exp.EndedAt.IsZero() {
						end = exp.EndedAt.Format("Jan 2006")
					}
					period = fmt.Sprintf("%s - %s", start, end)
				}
				roles = append(roles, map[string]interface{}{
					"id":      exp.ID,
					"company": exp.Organization,
					"role":    exp.RoleTitle,
					"period":  period,
				})
			}
		}
	}
	if len(roles) == 0 {
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
	}

	var contactParts []string
	if user.FullName != nil && *user.FullName != "" {
		contactParts = append(contactParts, *user.FullName)
	}
	if user.Phone != nil && *user.Phone != "" {
		contactParts = append(contactParts, *user.Phone)
	}
	if user.Email != nil && *user.Email != "" {
		contactParts = append(contactParts, *user.Email)
	}
	if user.LinkedInURL != nil && *user.LinkedInURL != "" {
		contactParts = append(contactParts, *user.LinkedInURL)
	}
	if user.GitHubURL != nil && *user.GitHubURL != "" {
		contactParts = append(contactParts, *user.GitHubURL)
	}
	contactLine := strings.Join(contactParts, " | ")

	facts := map[string]interface{}{
		"name":             utils.PtrStringOrEmpty(user.FullName),
		"headline":         utils.PtrStringOrEmpty(user.Headline),
		"phone":            utils.PtrStringOrEmpty(user.Phone),
		"email":            utils.PtrStringOrEmpty(user.Email),
		"linkedin":         utils.PtrStringOrEmpty(user.LinkedInURL),
		"github":           utils.PtrStringOrEmpty(user.GitHubURL),
		"portfolio":        utils.PtrStringOrEmpty(user.PortfolioURL),
		"education":        educationLine,
		"contact":          contactLine,
		"roles":            roles,
		"education_rows":   eduRows,
		"years_experience": candidate["years_experience"],
		"skills":           candidate["skills"],
		"experience":       candidate["experience"],
		"projects":         candidate["projects"],
	}

	input := map[string]interface{}{
		"evaluation_id":        id.String(),
		"job_id":               userJob.JobID,
		"user_id":              userJob.UserID.String(),
		"candidate_profile":    candidate,
		"facts":                facts,
		"job_profile":          job,
		"evaluation_results":   evidence,
		"overall_score":        utils.PtrFloatOrZero(eval.OverallScore),
		"eligibility_status":   utils.PtrStringOrEmpty(eval.EligibilityStatus),
		"recommendation_label": *eval.RecommendationLabel,
	}
	workflowID := fmt.Sprintf("resume-gen-%s-%d", id.String(), time.Now().UnixMilli())
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
	filename := "Resume.pdf"
	if err == nil && user != nil && user.FullName != nil && strings.TrimSpace(*user.FullName) != "" {
		cleanName := strings.ReplaceAll(strings.TrimSpace(*user.FullName), " ", "_")
		filename = fmt.Sprintf("%s_Resume.pdf", cleanName)
	}

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
		eval, err := s.triggerResume(ctx, item.ID, item.Force)
		if err != nil {
			message := err.Error()
			summaries = append(summaries, &models.MatchingEvaluationSummary{ID: &item.ID, Error: &message})
			continue
		}
		summaries = append(summaries, &models.MatchingEvaluationSummary{
			ID: &eval.ID, UserJobID: &eval.UserJobID, EvaluationStatus: eval.EvaluationStatus, CreatedAt: &eval.CreatedAt,
		})
	}
	return summaries, nil
}
