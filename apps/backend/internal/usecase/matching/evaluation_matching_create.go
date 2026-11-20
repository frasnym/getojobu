package matching

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// CreateEvaluations inserts one evaluation per resolved user of each job slug,
// at status "new". It performs no workflow start; the trigger step owns that.
// A row that already exists for a user/job is returned unchanged, whatever its
// status, so an insert never resets a success or in-progress evaluation.
func (s *matchingService) CreateEvaluations(
	ctx context.Context,
	items []models.CreateEvaluationItem,
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
			// Clerk caller: the user comes from the principal and ownership of
			// the job link is verified before an evaluation is created.
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
			// Machine caller: the key is trusted, so resolve every user linked
			// to the job from user_jobs rather than from the request.
			users, err := s.userJobsRepo.GetUsersByJobID(ctx, job.ID)
			if err != nil {
				errStr := fmt.Sprintf("get users by job id: %s", err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}
			targetUserIDs = users
		}

		jobProfile, err := DeriveJobProfile(job)
		if err != nil {
			errStr := fmt.Sprintf("parse job analyze_result: %s", err.Error())
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				Slug:  item.Slug,
				Error: &errStr,
			})
			continue
		}
		family, _ := jobProfile["family"].(string)

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
			if _, err := uuid.Parse(userIDStr); err != nil {
				errStr := fmt.Sprintf("invalid user id %s: %s", userIDStr, err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}

			// Resolving the user_job both yields its ID and verifies that the
			// user owns a link to this job.
			userJob, err := s.userJobsRepo.GetByUserAndJob(ctx, userIDStr, job.ID)
			if err != nil {
				errStr := fmt.Sprintf("get user_job for user %s: %s", userIDStr, err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}

			newRecord := &models.MatchingEvaluation{
				UserJobID:        userJob.ID,
				EvaluationStatus: string(constants.MatchingStatusNew),
				ScoringConfigID:  scoringConfig.ID,
			}

			eval, err := s.evalRepo.Create(ctx, newRecord)
			if err != nil {
				errStr := fmt.Sprintf("create matching evaluation: %s", err.Error())
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}

			summaries = append(summaries, &models.MatchingEvaluationSummary{
				Slug:             job.Slug,
				ID:               &eval.ID,
				UserJobID:        &eval.UserJobID,
				EvaluationStatus: eval.EvaluationStatus,
				CreatedAt:        &eval.CreatedAt,
			})
		}
	}

	return summaries, nil
}
