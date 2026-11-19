package matching

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// UpdateEvaluationReportRequest holds the payload sent by the worker to update a report.
type UpdateEvaluationReportRequest struct {
	ReportStatus         string          `json:"report_status"`
	ReportData           json.RawMessage `json:"report_data,omitempty"`
	ReportFailureMessage *string         `json:"report_failure_message,omitempty"`
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

		// A Clerk caller may only regenerate a report on an evaluation it owns.
		// Worker API keys keep the existing behavior and may target any
		// evaluation.
		if caller.ActorType == "" {
			owned, err := s.isEvaluationOwnedBy(ctx, item.ID, caller.ID)
			if err != nil {
				errStr := fmt.Sprintf("check evaluation ownership: %s", err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					ID:        &eval.ID,
					UserJobID: &eval.UserJobID,
					Error:     &errStr,
				})
				continue
			}
			if !owned {
				errStr := "not authorized to generate report for this evaluation"
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					ID:        &eval.ID,
					UserJobID: &eval.UserJobID,
					Error:     &errStr,
				})
				continue
			}
		}

		if eval.EvaluationStatus != string(constants.MatchingStatusSuccess) {
			errStr := fmt.Sprintf("cannot generate report: evaluation status is %s (must be success)", eval.EvaluationStatus)
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:           &eval.ID,
				UserJobID:    &eval.UserJobID,
				ReportStatus: eval.ReportStatus,
				Error:        &errStr,
			})
			continue
		}

		if eval.ReportStatus != nil && *eval.ReportStatus == string(constants.ReportStatusSuccess) && !item.Force {
			errStr := "report already generated, force must be true to regenerate"
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:           &eval.ID,
				UserJobID:    &eval.UserJobID,
				ReportStatus: eval.ReportStatus,
				Error:        &errStr,
			})
			continue
		}

		if eval.ReportStatus != nil && (*eval.ReportStatus == string(constants.ReportStatusQueued) || *eval.ReportStatus == string(constants.ReportStatusQueuedRetry) || *eval.ReportStatus == string(constants.ReportStatusPending)) && !item.Force {
			errStr := "report generation already in progress"
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:           &eval.ID,
				UserJobID:    &eval.UserJobID,
				ReportStatus: eval.ReportStatus,
				Error:        &errStr,
			})
			continue
		}

		// Mark report queued (or queued_retry if previously attempted)
		queueStatus := string(constants.ReportStatusQueued)
		if eval.ReportStatus != nil {
			queueStatus = string(constants.ReportStatusQueuedRetry)
		}
		updated, err := s.evalRepo.UpdateReport(ctx, eval.ID, models.UpdateMatchingEvaluationReportParams{
			ReportStatus: queueStatus,
		})
		if err != nil {
			errStr := fmt.Sprintf("mark report pending: %s", err.Error())
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:           &eval.ID,
				UserJobID:    &eval.UserJobID,
				ReportStatus: eval.ReportStatus,
				Error:        &errStr,
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
			"report_status":        queueStatus,
		}

		workflowID := fmt.Sprintf("report-gen-%s", eval.ID.String())
		if s.workflowsRepo != nil {
			if _, err := s.workflowsRepo.StartLimiterWorkItem(ctx, workflowID, models.LimiterJobTypeReport, wfInput); err != nil {
				errStr := fmt.Sprintf("start report workflow: %s", err.Error())
				_, _ = s.evalRepo.UpdateReport(ctx, eval.ID, models.UpdateMatchingEvaluationReportParams{
					ReportStatus:         string(constants.ReportStatusFailed),
					ReportFailureMessage: &errStr,
				})
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					ID:           &eval.ID,
					UserJobID:    &eval.UserJobID,
					ReportStatus: eval.ReportStatus,
					Error:        &errStr,
				})
				continue
			}
		}

		summaries = append(summaries, &models.MatchingEvaluationSummary{
			ID:           &eval.ID,
			UserJobID:    &eval.UserJobID,
			ReportStatus: eval.ReportStatus,
			CreatedAt:    &eval.CreatedAt,
		})
	}

	return summaries, nil
}
