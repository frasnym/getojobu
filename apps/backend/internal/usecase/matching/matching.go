package matching

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repo "github.com/frasnym/getojobu/apps/backend/internal/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// UpdateEvaluationRequest holds the payload sent by the worker to update an evaluation.
type UpdateEvaluationRequest struct {
	EvaluationStatus string  `json:"evaluation_status"`
	EvaluationData   []byte  `json:"evaluation_data,omitempty"`
	FailureMessage   *string `json:"failure_message,omitempty"`
}

// UpdateEvaluationReportRequest holds the payload sent by the worker to update a report.
type UpdateEvaluationReportRequest struct {
	ReportStatus         string          `json:"report_status"`
	ReportData           json.RawMessage `json:"report_data,omitempty"`
	ReportFailureMessage *string         `json:"report_failure_message,omitempty"`
}

// MatchJobWorkflowInput defines the payload passed to Temporal MatchJobWorkflow.
type MatchJobWorkflowInput struct {
	EvaluationID     string                 `json:"evaluation_id"`
	Slug             string                 `json:"slug"`
	CandidateProfile map[string]interface{} `json:"candidate_profile"`
	JobProfile       map[string]interface{} `json:"job_profile"`
	EvaluationConfig map[string]interface{} `json:"evaluation_config"`
}

// Matching defines operations for candidate-job compatibility evaluation.
type Matching interface {
	TriggerCompatibilityBulk(ctx context.Context, items []models.BulkAnalyzeCompatibilityItem, caller *models.Principal) ([]*models.MatchingEvaluationSummary, error)
	UpdateEvaluation(ctx context.Context, id uuid.UUID, req UpdateEvaluationRequest) (*models.MatchingEvaluation, error)
	UpdateEvaluationReport(ctx context.Context, id uuid.UUID, req UpdateEvaluationReportRequest) (*models.MatchingEvaluation, error)
	GetReport(ctx context.Context, id uuid.UUID, caller *models.Principal) (*models.MatchingEvaluationReportResponse, error)
	BulkQueueGenerateReport(ctx context.Context, items []models.BulkGenerateReportItem, caller *models.Principal) ([]*models.MatchingEvaluationSummary, error)
}

type matchingService struct {
	evalRepo           repo.MatchingEvaluations
	userJobsRepo       repo.UserJobs
	scoringConfigsRepo repo.ScoringConfigs
	jobsRepo           repo.Jobs
	expRepo            repo.Experiences
	projRepo           repo.Projects
	projSkillsRepo     repo.ProjectSkills
	workflowsRepo      repo.Workflows
}

// NewService constructs a new Matching usecase service.
func NewService(r *repository.Contract) Matching {
	return &matchingService{
		evalRepo:           r.MatchingEvaluationsRepository,
		userJobsRepo:       r.UserJobsRepository,
		scoringConfigsRepo: r.ScoringConfigsRepository,
		jobsRepo:           r.JobsRepository,
		expRepo:            r.ExperiencesRepository,
		projRepo:           r.ProjectsRepository,
		projSkillsRepo:     r.ProjectSkillsRepository,
		workflowsRepo:      r.WorkflowsRepository,
	}
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

		job, err := s.jobsRepo.GetBySlug(ctx, item.Slug)
		if err != nil {
			errStr := "job not found"
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				Slug:  item.Slug,
				Error: &errStr,
			})
			continue
		}

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

		// Parse job profile from AnalyzeResult to extract family
		var rawJobProfile map[string]interface{}
		if err := json.Unmarshal(job.AnalyzeResult, &rawJobProfile); err != nil {
			errStr := fmt.Sprintf("parse job analyze_result: %s", err.Error())
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				Slug:  item.Slug,
				Error: &errStr,
			})
			continue
		}
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

			var evalRecord *models.MatchingEvaluation
			var isRetry bool

			if err == nil && latestEval != nil {
				switch constants.MatchingEvaluationStatus(latestEval.EvaluationStatus) {
				case constants.MatchingStatusNew, constants.MatchingStatusPending, constants.MatchingStatusNewRetry:
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
					isRetry = true

				case constants.MatchingStatusFailed:
					if !item.Force {
						errStr := "evaluation failed, force must be true to re-evaluate"
						summaries = append(summaries, &models.MatchingEvaluationSummary{
							Slug:  item.Slug,
							Error: &errStr,
						})
						continue
					}
					// force=true on failed: reuse existing record, set status = new-retry, keep failure_message
					evalRecord = latestEval
					isRetry = true

				default:
					if !item.Force {
						errStr := fmt.Sprintf("evaluation status is %s, force must be true to re-evaluate", latestEval.EvaluationStatus)
						summaries = append(summaries, &models.MatchingEvaluationSummary{
							Slug:  item.Slug,
							Error: &errStr,
						})
						continue
					}
					isRetry = true
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
				// Reusing existing failed record -> status = new-retry
				updated, err := s.evalRepo.Update(ctx, evalRecord.ID, models.UpdateMatchingEvaluationParams{
					EvaluationStatus: string(constants.MatchingStatusNewRetry),
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
					ID:                   uuid.New(),
					UserJobID:            userJob.ID,
					EvaluationStatus:     string(constants.MatchingStatusNew),
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
			_ = json.Unmarshal(configBytes, &configMap)

			wfInput := MatchJobWorkflowInput{
				EvaluationID:     evalRecord.ID.String(),
				Slug:             job.Slug,
				CandidateProfile: candidateMap,
				JobProfile:       rawJobProfile,
				EvaluationConfig: configMap,
			}

			var workflowID string
			if isRetry {
				workflowID = fmt.Sprintf("match-job-%s-%d", evalRecord.ID.String(), time.Now().UnixMilli())
			} else {
				workflowID = fmt.Sprintf("match-job-%s", evalRecord.ID.String())
			}

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

		return updated, nil

	default:
		return nil, fmt.Errorf("%w: invalid status %s", constants.ErrInvalidRequest, req.EvaluationStatus)
	}
}

func (s *matchingService) UpdateEvaluationReport(
	ctx context.Context,
	id uuid.UUID,
	req UpdateEvaluationReportRequest,
) (*models.MatchingEvaluation, error) {
	eval, err := s.evalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	failMsg := req.ReportFailureMessage
	if eval.ReportFailureMessage != nil && *eval.ReportFailureMessage != "" && req.ReportFailureMessage != nil && *req.ReportFailureMessage != "" {
		combined := fmt.Sprintf("%s, %s", *eval.ReportFailureMessage, *req.ReportFailureMessage)
		failMsg = &combined
	}

	return s.evalRepo.UpdateReport(ctx, id, models.UpdateMatchingEvaluationReportParams{
		ReportStatus:         req.ReportStatus,
		ReportData:           req.ReportData,
		ReportFailureMessage: failMsg,
	})
}

func (s *matchingService) GetReport(
	ctx context.Context,
	id uuid.UUID,
	caller *models.Principal,
) (*models.MatchingEvaluationReportResponse, error) {
	if caller == nil {
		return nil, constants.ErrUnauthorized
	}

	eval, err := s.evalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Verify ownership if caller is a Clerk user (machine callers bypass)
	if caller.ActorType == "" {
		userJob, err := s.userJobsRepo.GetByID(ctx, eval.UserJobID)
		if err != nil {
			return nil, fmt.Errorf("get user job ownership: %w", err)
		}
		if userJob.UserID.String() != caller.ID {
			return nil, constants.ErrForbidden
		}
	}

	return &models.MatchingEvaluationReportResponse{
		ID:                   eval.ID,
		UserJobID:            eval.UserJobID,
		EvaluationStatus:     eval.EvaluationStatus,
		ReportStatus:         eval.ReportStatus,
		OverallScore:         eval.OverallScore,
		EligibilityStatus:    eval.EligibilityStatus,
		RecommendationLabel:  eval.RecommendationLabel,
		ReportData:           eval.ReportData,
		ReportFailureMessage: eval.ReportFailureMessage,
		CreatedAt:            eval.CreatedAt,
		UpdatedAt:            eval.UpdatedAt,
	}, nil
}

func (s *matchingService) BulkQueueGenerateReport(
	ctx context.Context,
	items []models.BulkGenerateReportItem,
	caller *models.Principal,
) ([]*models.MatchingEvaluationSummary, error) {
	if caller == nil {
		return nil, constants.ErrUnauthorized
	}

	var summaries []*models.MatchingEvaluationSummary

	for _, item := range items {
		eval, err := s.evalRepo.GetByID(ctx, item.ID)
		if err != nil {
			errStr := "evaluation not found"
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:    &item.ID,
				Error: &errStr,
			})
			continue
		}

		if eval.EvaluationStatus != string(constants.MatchingStatusSuccess) {
			errStr := fmt.Sprintf("cannot generate report: evaluation status is %s (must be success)", eval.EvaluationStatus)
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:               &eval.ID,
				UserJobID:        &eval.UserJobID,
				EvaluationStatus: eval.EvaluationStatus,
				Error:            &errStr,
			})
			continue
		}

		if eval.ReportStatus != nil && *eval.ReportStatus == string(constants.ReportStatusGenerated) && !item.Force {
			errStr := "report already generated, force must be true to regenerate"
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:               &eval.ID,
				UserJobID:        &eval.UserJobID,
				EvaluationStatus: eval.EvaluationStatus,
				Error:            &errStr,
			})
			continue
		}

		if eval.ReportStatus != nil && (*eval.ReportStatus == string(constants.ReportStatusPending) || *eval.ReportStatus == string(constants.ReportStatusPendingRetry)) && !item.Force {
			errStr := "report generation already in progress"
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:               &eval.ID,
				UserJobID:        &eval.UserJobID,
				EvaluationStatus: eval.EvaluationStatus,
				Error:            &errStr,
			})
			continue
		}

		// Mark report pending (or pending-retry if previously attempted)
		pendingStatus := string(constants.ReportStatusPending)
		if eval.ReportStatus != nil {
			pendingStatus = string(constants.ReportStatusPendingRetry)
		}
		updated, err := s.evalRepo.UpdateReport(ctx, eval.ID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: pendingStatus,
		})
		if err != nil {
			errStr := fmt.Sprintf("mark report pending: %s", err.Error())
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:               &eval.ID,
				UserJobID:        &eval.UserJobID,
				EvaluationStatus: eval.EvaluationStatus,
				Error:            &errStr,
			})
			continue
		}
		eval = updated

		// Prepare workflow input
		var candidateMap, jobMap, evalResults map[string]interface{}
		_ = json.Unmarshal(eval.CandidateProfileData, &candidateMap)
		_ = json.Unmarshal(eval.JobProfileData, &jobMap)
		_ = json.Unmarshal(eval.EvaluationData, &evalResults)

		var overallScore float64
		if eval.OverallScore != nil {
			overallScore = *eval.OverallScore
		}
		var eligStatus string
		if eval.EligibilityStatus != nil {
			eligStatus = *eval.EligibilityStatus
		}
		var recLabel string
		if eval.RecommendationLabel != nil {
			recLabel = *eval.RecommendationLabel
		}

		wfInput := map[string]interface{}{
			"evaluation_id":        eval.ID.String(),
			"candidate_profile":    candidateMap,
			"job_profile":          jobMap,
			"evaluation_results":   evalResults,
			"overall_score":        overallScore,
			"eligibility_status":   eligStatus,
			"recommendation_label": recLabel,
			"report_status":        pendingStatus,
		}

		workflowID := fmt.Sprintf("report-gen-%s-%d", eval.ID.String(), time.Now().UnixMilli())
		if s.workflowsRepo != nil {
			if _, err := s.workflowsRepo.StartLimiterWorkItem(ctx, workflowID, models.LimiterJobTypeReport, wfInput); err != nil {
				errStr := fmt.Sprintf("start report workflow: %s", err.Error())
				_, _ = s.evalRepo.UpdateReport(ctx, eval.ID, models.UpdateMatchingEvaluationReportParams{
					ReportStatus:         string(constants.ReportStatusFailed),
					ReportFailureMessage: &errStr,
				})
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					ID:               &eval.ID,
					UserJobID:        &eval.UserJobID,
					EvaluationStatus: eval.EvaluationStatus,
					Error:            &errStr,
				})
				continue
			}
		}

		summaries = append(summaries, &models.MatchingEvaluationSummary{
			ID:               &eval.ID,
			UserJobID:        &eval.UserJobID,
			EvaluationStatus: eval.EvaluationStatus,
			CreatedAt:        &eval.CreatedAt,
		})
	}

	return summaries, nil
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
	}, nil
}
