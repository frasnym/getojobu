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
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
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

func (s *matchingService) TriggerCompatibilityBulk(
	ctx context.Context,
	items []models.BulkAnalyzeCompatibilityItem,
	caller *models.Principal,
) ([]*models.MatchingEvaluationSummary, error) {
	var summaries []*models.MatchingEvaluationSummary

	for _, item := range items {
		if item.Slug == "" {
			errStr := "slug is required"
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				Slug:  item.Slug,
				Error: &errStr,
			})
			continue
		}

		jobWithOwnership, err := s.jobsRepo.GetBySlug(ctx, item.Slug, "")
		if err != nil {
			errStr := "job not found"
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				Slug:  item.Slug,
				Error: &errStr,
			})
			continue
		}
		job := jobWithOwnership.Job

		if len(job.AnalyzeResult) == 0 {
			errStr := fmt.Sprintf("job %q is not analyzed", item.Slug)
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				Slug:  item.Slug,
				Error: &errStr,
			})
			continue
		}

		var targetUserIDs []string
		if caller != nil && caller.ActorType == "" {
			// Clerk user
			owned, err := s.userJobsRepo.IsOwned(ctx, caller.ID, job.ID)
			if err != nil || !owned {
				errStr := "job link not found"
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}
			targetUserIDs = []string{caller.ID}
		} else {
			// Machine worker
			users, err := s.userJobsRepo.GetUsersByJobID(ctx, job.ID)
			if err != nil {
				errStr := fmt.Sprintf("get users by job id: %s", err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}
			if len(users) == 0 {
				// For background machine worker, no linked users means nothing to evaluate yet
				continue
			}
			targetUserIDs = users
		}

		// Parse job profile from AnalyzeResult to extract family. Service
		// metadata (model/prompt_version) stays in jobs.analyze_result but
		// must not flow into the match workflow.
		var rawJobProfile map[string]interface{}
		if err := json.Unmarshal(utils.StripJSONKeys(job.AnalyzeResult, "metadata"), &rawJobProfile); err != nil {
			errStr := fmt.Sprintf("parse job analyze_result: %s", err.Error())
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				Slug:  item.Slug,
				Error: &errStr,
			})
			continue
		}
		// Append company/title/location from the shared jobs table so the
		// downstream evaluation still has them even though the LLM no longer
		// re-extracts them (they are redundant with jobs.title/company/location).
		rawJobProfile["title"] = job.Title
		rawJobProfile["company"] = job.Company
		rawJobProfile["location"] = job.Location
		family, _ := rawJobProfile["family"].(string)
		if family == "" {
			family = "other"
		}

		scoringConfig, err := s.scoringConfigsRepo.GetActiveByFamily(ctx, family)
		if err != nil {
			var fallbackErr error
			scoringConfig, fallbackErr = s.scoringConfigsRepo.GetActiveByFamily(ctx, "technology")
			if fallbackErr != nil {
				errStr := fmt.Sprintf("get active scoring config: %s", err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}
		}

		for _, userIDStr := range targetUserIDs {
			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				errStr := fmt.Sprintf("invalid user id %s: %s", userIDStr, err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}

			userJob, err := s.userJobsRepo.GetByUserAndJob(ctx, userIDStr, job.ID)
			if err != nil {
				errStr := fmt.Sprintf("get user_job for user %s: %s", userIDStr, err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}

			// State machine & force rules check
			latestEval, err := s.evalRepo.GetLatestByUserAndJob(ctx, userID, job.ID)
			if err != nil && !errors.Is(err, constants.ErrNotFound) {
				errStr := fmt.Sprintf("get latest evaluation: %s", err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}

			// A failed-only retry must not touch users whose latest evaluation
			// is healthy: force=true would create a new record and hide their
			// existing report/resume/cover letter behind a fresh queued row.
			if item.FailedOnly && (err != nil || latestEval == nil || latestEval.EvaluationStatus != string(constants.MatchingStatusFailed)) {
				continue
			}

			var evalRecord *models.MatchingEvaluation

			if err == nil && latestEval != nil {
				switch constants.MatchingEvaluationStatus(latestEval.EvaluationStatus) {
				case constants.MatchingStatusQueued, constants.MatchingStatusQueuedRetry, constants.MatchingStatusPending:
					errStr := fmt.Sprintf("evaluation is already in progress with status %s", latestEval.EvaluationStatus)
					summaries = append(summaries, &models.MatchingEvaluationSummary{
						Slug:  item.Slug,
						Error: &errStr,
					})
					continue

				case constants.MatchingStatusSuccess:
					if !item.Force {
						errStr := "job already evaluated with status success, force must be true to re-evaluate"
						summaries = append(summaries, &models.MatchingEvaluationSummary{
							Slug:  item.Slug,
							Error: &errStr,
						})
						continue
					}
					// force=true on success: create new record with uuid.New()

				case constants.MatchingStatusFailed:
					if !item.Force {
						errStr := "evaluation failed, force must be true to re-evaluate"
						summaries = append(summaries, &models.MatchingEvaluationSummary{
							Slug:  item.Slug,
							Error: &errStr,
						})
						continue
					}
					// force=true on failed: reuse existing record, set status = queued_retry, keep failure_message
					evalRecord = latestEval

				default:
					if !item.Force {
						errStr := fmt.Sprintf("evaluation status is %s, force must be true to re-evaluate", latestEval.EvaluationStatus)
						summaries = append(summaries, &models.MatchingEvaluationSummary{
							Slug:  item.Slug,
							Error: &errStr,
						})
						continue
					}
				}
			}

			candidateProfile, err := s.composeCandidateProfile(ctx, userIDStr)
			if err != nil {
				errStr := fmt.Sprintf("compose candidate profile: %s", err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}

			candidateJSON, err := json.Marshal(candidateProfile)
			if err != nil {
				errStr := fmt.Sprintf("marshal candidate profile: %s", err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}

			if evalRecord != nil {
				// Reusing existing failed record -> status = queued_retry
				updated, err := s.evalRepo.Update(ctx, evalRecord.ID, models.UpdateMatchingEvaluationParams{
					EvaluationStatus: string(constants.MatchingStatusQueuedRetry),
				})
				if err != nil {
					errStr := fmt.Sprintf("update matching evaluation: %s", err.Error())
					summaries = append(summaries, &models.MatchingEvaluationSummary{
						Slug:  item.Slug,
						Error: &errStr,
					})
					continue
				}
				evalRecord = updated
			} else {
				// Create new record (first-time or force-re-evaluating a success record)
				newRecord := &models.MatchingEvaluation{
					UserJobID:            userJob.ID,
					EvaluationStatus:     string(constants.MatchingStatusQueued),
					CandidateProfileData: candidateJSON,
					JobProfileData:       job.AnalyzeResult,
					ScoringConfigID:      scoringConfig.ID,
				}

				created, err := s.evalRepo.Create(ctx, newRecord)
				if err != nil {
					errStr := fmt.Sprintf("create matching evaluation: %s", err.Error())
					summaries = append(summaries, &models.MatchingEvaluationSummary{
						Slug:  item.Slug,
						Error: &errStr,
					})
					continue
				}
				evalRecord = created
			}

			// Prepare workflow input
			var candidateMap map[string]interface{}
			_ = json.Unmarshal(candidateJSON, &candidateMap)

			var configMap map[string]interface{}
			configBytes, _ := json.Marshal(scoringConfig)
			// Strip scoring-config row metadata from what is sent to Temporal —
			// the worker only needs family and categories, and the row's id,
			// version, is_active flag, and timestamps are not part of the
			// evaluation contract.
			_ = json.Unmarshal(utils.StripJSONKeys(configBytes, "id", "version", "is_active", "created_at", "updated_at"), &configMap)

			wfInput := MatchJobWorkflowInput{
				EvaluationID:     evalRecord.ID.String(),
				Slug:             job.Slug,
				CandidateProfile: candidateMap,
				JobProfile:       rawJobProfile,
				EvaluationConfig: configMap,
			}

			workflowID := fmt.Sprintf("match-job-%s", evalRecord.ID.String())

			if s.workflowsRepo != nil {
				if _, err := s.workflowsRepo.StartLimiterWorkItem(ctx, workflowID, models.LimiterJobTypeMatch, wfInput); err != nil {
					errStr := fmt.Sprintf("start match job workflow: %s", err.Error())
					summaries = append(summaries, &models.MatchingEvaluationSummary{
						Slug:  item.Slug,
						Error: &errStr,
					})
					continue
				}
			}

			summaries = append(summaries, &models.MatchingEvaluationSummary{
				Slug:             job.Slug,
				ID:               &evalRecord.ID,
				UserJobID:        &evalRecord.UserJobID,
				EvaluationStatus: evalRecord.EvaluationStatus,
				CreatedAt:        &evalRecord.CreatedAt,
			})
		}
	}

	return summaries, nil
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
