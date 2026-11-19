package matching

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// UpdateEvaluationRequest holds the payload sent by the worker to update an evaluation.
type UpdateEvaluationRequest struct {
	EvaluationStatus string  `json:"evaluation_status"`
	EvaluationData   []byte  `json:"evaluation_data,omitempty"`
	FailureMessage   *string `json:"failure_message,omitempty"`
}

// MatchJobWorkflowInput defines the payload passed to Temporal MatchJobWorkflow.
type MatchJobWorkflowInput struct {
	EvaluationID     string                 `json:"evaluation_id"`
	Slug             string                 `json:"slug"`
	CandidateProfile map[string]interface{} `json:"candidate_profile"`
	JobProfile       map[string]interface{} `json:"job_profile"`
	EvaluationConfig map[string]interface{} `json:"evaluation_config"`
}

// CalculateScores calculates the weighted overall score, aggregates mandatory requirements
// to determine eligibility, and derives the deterministic recommendation label.
func CalculateScores(
	configCategories []models.CategoryConfig,
	output models.MatchEvaluationOutput,
) (float64, constants.EligibilityStatus, constants.RecommendationLabel, error) {
	evalMap := make(map[string]models.CategoryEvaluation)
	for _, ev := range output.Evaluations {
		evalMap[ev.Category] = ev
	}

	var totalScore, totalWeight float64
	for _, cat := range configCategories {
		ev, exists := evalMap[cat.Name]
		if !exists {
			return 0, "", "", fmt.Errorf("missing category %s in evaluations", cat.Name)
		}
		totalScore += float64(ev.Score) * cat.Weight
		totalWeight += cat.Weight
	}

	if totalWeight <= 0 {
		return 0, "", "", errors.New("total weight of scoring categories must be positive")
	}

	overallScore := math.Round((totalScore/totalWeight)*100) / 100

	// Aggregate mandatory requirements to determine eligibility
	hasNotMet := false
	hasUncertain := false
	for _, req := range output.MandatoryRequirements {
		switch constants.RequirementStatus(req.Status) {
		case constants.RequirementStatusNotMet:
			hasNotMet = true
		case constants.RequirementStatusUncertain:
			hasUncertain = true
		}
	}

	var eligibility constants.EligibilityStatus
	if hasNotMet {
		eligibility = constants.EligibilityStatusNotEligible
	} else if hasUncertain {
		eligibility = constants.EligibilityStatusNeedsVerification
	} else {
		eligibility = constants.EligibilityStatusEligible
	}

	// Deterministic recommendation label
	var recommendation constants.RecommendationLabel
	if overallScore >= 75 && eligibility == constants.EligibilityStatusEligible {
		recommendation = constants.RecommendationStrongMatch
	} else if overallScore >= 60 && eligibility == constants.EligibilityStatusEligible {
		recommendation = constants.RecommendationWorthApplying
	} else if overallScore >= 40 && eligibility != constants.EligibilityStatusNotEligible {
		recommendation = constants.RecommendationPossibleMatch
	} else {
		recommendation = constants.RecommendationLowMatch
	}

	return overallScore, eligibility, recommendation, nil
}

func (s *matchingService) UpdateEvaluation(
	ctx context.Context,
	id uuid.UUID,
	req UpdateEvaluationRequest,
) (*models.MatchingEvaluation, error) {
	eval, err := s.evalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := constants.ValidateMatchingStatusTransition(
		constants.MatchingEvaluationStatus(eval.EvaluationStatus),
		constants.MatchingEvaluationStatus(req.EvaluationStatus),
	); err != nil {
		return nil, fmt.Errorf("%w: %s", constants.ErrInvalidTransition, err.Error())
	}

	switch constants.MatchingEvaluationStatus(req.EvaluationStatus) {
	case constants.MatchingStatusPending:
		return s.evalRepo.Update(ctx, id, models.UpdateMatchingEvaluationParams{
			EvaluationStatus: string(constants.MatchingStatusPending),
		})

	case constants.MatchingStatusFailed:
		failMsg := req.FailureMessage
		if eval.FailureMessage != nil && *eval.FailureMessage != "" && req.FailureMessage != nil && *req.FailureMessage != "" {
			combined := fmt.Sprintf("%s, %s", *eval.FailureMessage, *req.FailureMessage)
			failMsg = &combined
		}
		return s.evalRepo.Update(ctx, id, models.UpdateMatchingEvaluationParams{
			EvaluationStatus: string(constants.MatchingStatusFailed),
			FailureMessage:   failMsg,
		})

	case constants.MatchingStatusSuccess:
		var output models.MatchEvaluationOutput
		if err := json.Unmarshal(req.EvaluationData, &output); err != nil {
			return nil, fmt.Errorf("%w: invalid evaluation_data json: %s", constants.ErrInvalidRequest, err.Error())
		}

		scoringConfig, err := s.scoringConfigsRepo.GetByID(ctx, eval.ScoringConfigID)
		if err != nil {
			return nil, fmt.Errorf("get scoring config: %w", err)
		}

		if err := models.ValidateEvaluationOutput(scoringConfig.Categories, output); err != nil {
			return nil, fmt.Errorf("%w: %s", constants.ErrInvalidRequest, err.Error())
		}

		overallScore, eligibility, recommendation, err := CalculateScores(scoringConfig.Categories, output)
		if err != nil {
			return nil, fmt.Errorf("calculate scores: %w", err)
		}

		eligStr := string(eligibility)
		recStr := string(recommendation)

		updated, err := s.evalRepo.Update(ctx, id, models.UpdateMatchingEvaluationParams{
			EvaluationStatus:    string(constants.MatchingStatusSuccess),
			EvaluationData:      req.EvaluationData,
			OverallScore:        &overallScore,
			EligibilityStatus:   &eligStr,
			RecommendationLabel: &recStr,
		})
		if err != nil {
			return nil, fmt.Errorf("update matching evaluation success: %w", err)
		}

		if recommendation == constants.RecommendationLowMatch {
			if err := s.autoHideLowMatch(ctx, eval.UserJobID); err != nil {
				return nil, fmt.Errorf("auto hide low match evaluation: %w", err)
			}
		}

		return updated, nil

	default:
		return nil, fmt.Errorf("%w: invalid status %s", constants.ErrInvalidRequest, req.EvaluationStatus)
	}
}

// autoHideLowMatch hides the user's job when they opted into automatic hiding
// of low-match evaluations. The setting is read from any active preference row
// for the user, so it does not depend on which search set produced the job.
func (s *matchingService) autoHideLowMatch(ctx context.Context, userJobID uuid.UUID) error {
	userJob, err := s.userJobsRepo.GetByID(ctx, userJobID)
	if err != nil {
		return fmt.Errorf("get user job: %w", err)
	}

	userID := userJob.UserID.String()
	enabled, err := s.userPreferencesRepo.IsAutoHideLowMatchEnabled(ctx, userID)
	if err != nil {
		return fmt.Errorf("check auto hide preference: %w", err)
	}
	if !enabled {
		return nil
	}

	if _, err := s.userJobsRepo.SetBulkHideStatus(ctx, userID, []string{userJobID.String()}, true); err != nil {
		return fmt.Errorf("set hide status: %w", err)
	}
	return nil
}

func (s *matchingService) composeCandidateProfile(ctx context.Context, userID string) (map[string]interface{}, error) {
	exps, err := s.expRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	projs, err := s.projRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	skills, err := s.projSkillsRepo.ListSkillsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	skillNames := make([]string, 0, len(skills))
	for _, sk := range skills {
		skillNames = append(skillNames, sk.SkillName)
	}

	// Assemble education from user_educations table so it is included in
	// candidate_profile_data passed to the match workflow.
	var educationLine string
	var eduRows []*models.UserEducation
	if s.eduRepo != nil {
		edus, err := s.eduRepo.ListByUser(ctx, userID)
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

	expList := make([]map[string]interface{}, 0, len(exps))
	var totalDays float64
	for _, e := range exps {
		item := map[string]interface{}{
			"company":     e.Organization,
			"role":        e.RoleTitle,
			"location":    e.Location,
			"description": e.Description,
		}
		if !e.StartedAt.IsZero() {
			item["started_at"] = e.StartedAt.Format("2006-01-02")
			end := time.Now()
			if !e.EndedAt.IsZero() {
				end = e.EndedAt
				item["ended_at"] = e.EndedAt.Format("2006-01-02")
				item["is_current"] = false
			} else {
				item["is_current"] = true
			}
			if end.After(e.StartedAt) {
				totalDays += end.Sub(e.StartedAt).Hours() / 24.0
			}
		}
		expList = append(expList, item)
	}

	yearsExp := int(math.Round(totalDays / 365.25))

	projList := make([]map[string]interface{}, 0, len(projs))
	for _, p := range projs {
		projList = append(projList, map[string]interface{}{
			"name":             p.Name,
			"description":      p.Description,
			"responsibilities": p.Responsibilities,
			"achievements":     p.Achievements,
		})
	}

	return map[string]interface{}{
		"years_experience": yearsExp,
		"skills":           skillNames,
		"experience":       expList,
		"projects":         projList,
		"education":        educationLine,
		"education_rows":   eduRows,
	}, nil
}
