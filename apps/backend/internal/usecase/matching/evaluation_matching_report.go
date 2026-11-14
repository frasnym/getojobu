package matching

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

		if eval.ReportStatus != nil && *eval.ReportStatus == string(constants.ReportStatusSuccess) && !item.Force {
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
