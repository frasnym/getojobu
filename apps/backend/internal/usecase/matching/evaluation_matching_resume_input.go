package matching

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

// resumeInputData is the snapshot the resume worker needs. Both the queue
// step and the worker-facing GET build it through buildResumeInput so they
// use the same profile and facts.
type resumeInputData struct {
	CandidateProfile    map[string]interface{}
	JobProfile          map[string]interface{}
	EvaluationResults   map[string]interface{}
	Facts               map[string]interface{}
	OverallScore        float64
	EligibilityStatus   string
	RecommendationLabel string
	JobID               int64
	UserID              string
}

func (s *matchingService) buildResumeInput(
	ctx context.Context,
	eval *models.MatchingEvaluation,
	userJob *models.UserJob,
	user *models.User,
) (*resumeInputData, error) {
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

	// Assemble education from user_educations table (pre-formatted lines for
	// all educations — keep the payload small, no raw DB rows in facts; the
	// full array is mirrored into candidate_profile.educations).
	var educationLine string
	var eduRows []*models.UserEducation
	if s.eduRepo != nil {
		edus, err := s.eduRepo.ListByUser(ctx, user.ID)
		if err == nil && len(edus) > 0 {
			eduRows = edus
			var lines []string
			for _, top := range edus {
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
				lines = append(lines, strings.Join(parts, " | "))
			}
			educationLine = strings.Join(lines, "\n")
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
	for _, value := range []*string{user.FullName, user.Phone, user.Email, user.LinkedInURL, user.GitHubURL} {
		if value != nil && strings.TrimSpace(*value) != "" {
			contactParts = append(contactParts, *value)
		}
	}

	// The worker finds education data alongside the rest of the candidate
	// profile, not only inside facts.
	candidate["educations"] = eduRows

	facts := map[string]interface{}{
		"name":      utils.PtrStringOrEmpty(user.FullName),
		"headline":  utils.PtrStringOrEmpty(user.Headline),
		"phone":     utils.PtrStringOrEmpty(user.Phone),
		"email":     utils.PtrStringOrEmpty(user.Email),
		"linkedin":  utils.PtrStringOrEmpty(user.LinkedInURL),
		"github":    utils.PtrStringOrEmpty(user.GitHubURL),
		"portfolio": utils.PtrStringOrEmpty(user.PortfolioURL),
		"education": educationLine,
		"contact":   strings.Join(contactParts, " | "),
		"roles":     roles,
	}

	return &resumeInputData{
		CandidateProfile:    candidate,
		JobProfile:          job,
		EvaluationResults:   evidence,
		Facts:               facts,
		OverallScore:        utils.PtrFloatOrZero(eval.OverallScore),
		EligibilityStatus:   utils.PtrStringOrEmpty(eval.EligibilityStatus),
		RecommendationLabel: utils.PtrStringOrEmpty(eval.RecommendationLabel),
		JobID:               userJob.JobID,
		UserID:              userJob.UserID.String(),
	}, nil
}
