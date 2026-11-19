package matching

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// TriggerEvaluations starts the Temporal workflow behind each evaluation that
// is ready to run. A new row moves to queued; a failed row is retried in place
// with one atomic clear-and-flip and moves to queued_retry. Every workflow is
// started before its status is flipped, so a queued row never exists without a
// workflow behind it.
func (s *matchingService) TriggerEvaluations(
	ctx context.Context,
	items []models.TriggerEvaluationItem,
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

		if caller.ActorType == "" {
			// A Clerk caller's user comes from the principal; ownership of the
			// evaluation's user_job is still checked. API key callers are
			// trusted and need no per-item user check.
			userJob, err := s.userJobsRepo.GetByID(ctx, eval.UserJobID)
			if err != nil {
				errStr := fmt.Sprintf("get user job: %s", err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					ID:        &eval.ID,
					UserJobID: &eval.UserJobID,
					Error:     &errStr,
				})
				continue
			}
			if userJob.UserID.String() != caller.ID {
				errStr := "evaluation belongs to another user"
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					ID:        &eval.ID,
					UserJobID: &eval.UserJobID,
					Error:     &errStr,
				})
				continue
			}
		}

		var from, to constants.MatchingEvaluationStatus
		switch constants.MatchingEvaluationStatus(eval.EvaluationStatus) {
		case constants.MatchingStatusNew:
			from, to = constants.MatchingStatusNew, constants.MatchingStatusQueued
		case constants.MatchingStatusFailed:
			from, to = constants.MatchingStatusFailed, constants.MatchingStatusQueuedRetry
		default:
			errStr := fmt.Sprintf("cannot trigger evaluation with status %s", eval.EvaluationStatus)
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:               &eval.ID,
				UserJobID:        &eval.UserJobID,
				EvaluationStatus: eval.EvaluationStatus,
				Error:            &errStr,
			})
			continue
		}

		scoringConfig, err := s.scoringConfigsRepo.GetByID(ctx, eval.ScoringConfigID)
		if err != nil {
			errStr := fmt.Sprintf("get scoring config: %s", err.Error())
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:        &eval.ID,
				UserJobID: &eval.UserJobID,
				Error:     &errStr,
			})
			continue
		}

		// Decode the stored profile bytes and scoring config into memory before
		// starting the workflow. Profiles are inputs and survive retries.
		var candidateMap, jobMap, configMap map[string]interface{}
		_ = json.Unmarshal(eval.CandidateProfileData, &candidateMap)
		_ = json.Unmarshal(eval.JobProfileData, &jobMap)
		configBytes, _ := json.Marshal(scoringConfig)
		_ = json.Unmarshal(utils.StripJSONKeys(configBytes, "id", "version", "is_active", "created_at", "updated_at"), &configMap)

		wfInput := MatchJobWorkflowInput{
			EvaluationID:     eval.ID.String(),
			CandidateProfile: candidateMap,
			JobProfile:       jobMap,
			EvaluationConfig: configMap,
		}
		workflowID := fmt.Sprintf("match-job-%s", eval.ID.String())
		if _, err := s.workflowsRepo.StartLimiterWorkItem(ctx, workflowID, models.LimiterJobTypeMatch, wfInput); err != nil {
			errStr := fmt.Sprintf("start match job workflow: %s", err.Error())
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:        &eval.ID,
				UserJobID: &eval.UserJobID,
				Error:     &errStr,
			})
			continue
		}

		var flipped bool
		if from == constants.MatchingStatusFailed {
			flipped, err = s.evalRepo.FlipStatusWithClear(ctx, eval.ID, string(from), string(to))
		} else {
			flipped, err = s.evalRepo.FlipStatus(ctx, eval.ID, string(from), string(to))
		}
		if err != nil {
			errStr := fmt.Sprintf("flip evaluation status: %s", err.Error())
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:        &eval.ID,
				UserJobID: &eval.UserJobID,
				Error:     &errStr,
			})
			continue
		}
		if !flipped {
			// The worker may have moved the row to pending before our
			// conditional update landed, or a prior trigger may already have
			// queued it. In either case the workflow is running, so report the
			// in-flight state as a success instead of a false failure.
			if current, getErr := s.evalRepo.GetByID(ctx, eval.ID); getErr == nil &&
				isInFlightEvaluationStatus(constants.MatchingEvaluationStatus(current.EvaluationStatus)) {
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					ID:               &eval.ID,
					UserJobID:        &eval.UserJobID,
					EvaluationStatus: current.EvaluationStatus,
					CreatedAt:        &eval.CreatedAt,
				})
				continue
			}
			errStr := "evaluation status changed before it could be queued"
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				ID:        &eval.ID,
				UserJobID: &eval.UserJobID,
				Error:     &errStr,
			})
			continue
		}

		summaries = append(summaries, &models.MatchingEvaluationSummary{
			ID:               &eval.ID,
			UserJobID:        &eval.UserJobID,
			EvaluationStatus: string(to),
			CreatedAt:        &eval.CreatedAt,
		})
	}

	return summaries, nil
}

// isInFlightEvaluationStatus reports whether a status means a workflow is
// already running for the row, so a failed conditional status flip is not a
// failure when the row is one of these states.
func isInFlightEvaluationStatus(status constants.MatchingEvaluationStatus) bool {
	switch status {
	case constants.MatchingStatusPending, constants.MatchingStatusQueued, constants.MatchingStatusQueuedRetry:
		return true
	default:
		return false
	}
}
