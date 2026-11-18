package matching

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
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
			// Clerk caller: the user comes from the principal. A caller cannot
			// create an evaluation on behalf of a user it supplies in the body.
			if len(item.UserIDs) > 0 {
				errStr := "user_ids are not allowed for clerk callers"
				summaries = append(summaries, &models.MatchingEvaluationSummary{
					Slug:  item.Slug,
					Error: &errStr,
				})
				continue
			}
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
			// Machine caller: user IDs are supplied per item; ownership of the
			// job link is verified when each user_job is resolved below.
			targetUserIDs = item.UserIDs
		}

		// Build the derived job profile: strip service metadata, default the
		// family, and copy title/company/location from the shared jobs table.
		// This is what gets stored and later sent to Temporal by the trigger.
		var rawJobProfile map[string]interface{}
		if err := json.Unmarshal(utils.StripJSONKeys(job.AnalyzeResult, "metadata"), &rawJobProfile); err != nil {
			errStr := fmt.Sprintf("parse job analyze_result: %s", err.Error())
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				Slug:  item.Slug,
				Error: &errStr,
			})
			continue
		}
		rawJobProfile["title"] = job.Title
		rawJobProfile["company"] = job.Company
		rawJobProfile["location"] = job.Location
		family, _ := rawJobProfile["family"].(string)
		if family == "" {
			family = "other"
			rawJobProfile["family"] = family
		}

		derivedJobProfile, err := json.Marshal(rawJobProfile)
		if err != nil {
			errStr := fmt.Sprintf("marshal job profile: %s", err.Error())
			summaries = append(summaries, &models.MatchingEvaluationSummary{
				Slug:  item.Slug,
				Error: &errStr,
			})
			continue
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

			newRecord := &models.MatchingEvaluation{
				UserJobID:            userJob.ID,
				EvaluationStatus:     string(constants.MatchingStatusNew),
				CandidateProfileData: candidateJSON,
				JobProfileData:       derivedJobProfile,
				ScoringConfigID:      scoringConfig.ID,
			}

			eval, _, err := s.evalRepo.CreateIfAbsent(ctx, newRecord)
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
