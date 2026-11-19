package matching

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

// GetEvaluation returns the worker-facing snapshot for an evaluation. The
// candidate profile and facts come from the user cache (composed once when
// missing); the job profile is derived from the shared jobs table.
func (s *matchingService) GetEvaluation(ctx context.Context, id uuid.UUID) (*models.GetEvaluationResponse, error) {
	eval, err := s.evalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	config, err := s.scoringConfigsRepo.GetByID(ctx, eval.ScoringConfigID)
	if err != nil {
		return nil, err
	}
	userJob, err := s.userJobsRepo.GetByID(ctx, eval.UserJobID)
	if err != nil {
		return nil, err
	}
	user, err := s.usersRepo.FindByID(ctx, userJob.UserID.String())
	if err != nil {
		return nil, err
	}
	job, err := s.jobsRepo.GetByID(ctx, userJob.JobID)
	if err != nil {
		return nil, err
	}

	var candidate, facts map[string]interface{}
	if len(user.ProfileData) > 0 {
		_ = json.Unmarshal(user.ProfileData, &candidate)
	}
	if len(user.FactsData) > 0 {
		_ = json.Unmarshal(user.FactsData, &facts)
	}
	if candidate == nil || facts == nil {
		if candidate == nil {
			candidate, err = s.composeCandidateProfile(ctx, user.ID)
			if err != nil {
				return nil, err
			}
		}
		facts, candidate = s.composeFacts(ctx, user, candidate)
		profileJSON, _ := json.Marshal(candidate)
		factsJSON, _ := json.Marshal(facts)
		if err := s.usersRepo.UpdateProfileCache(ctx, user.ID, profileJSON, factsJSON); err != nil {
			return nil, err
		}
	}
	if candidate == nil {
		candidate = map[string]interface{}{}
	}
	if facts == nil {
		facts = map[string]interface{}{}
	}

	jobProfile, err := deriveJobProfile(job)
	if err != nil {
		return nil, err
	}

	var evidence map[string]interface{}
	_ = json.Unmarshal(eval.EvaluationData, &evidence)
	if evidence == nil {
		evidence = map[string]interface{}{}
	}

	configBytes, _ := json.Marshal(config)
	var configMap map[string]interface{}
	_ = json.Unmarshal(utils.StripJSONKeys(configBytes, "id", "version", "is_active", "created_at", "updated_at"), &configMap)
	if configMap == nil {
		configMap = map[string]interface{}{}
	}

	return &models.GetEvaluationResponse{
		EvaluationStatus:    eval.EvaluationStatus,
		ReportStatus:        utils.PtrStringOrEmpty(eval.ReportStatus),
		ResumeStatus:        utils.PtrStringOrEmpty(eval.ResumeStatus),
		CandidateProfile:    candidate,
		JobProfile:          jobProfile,
		EvaluationResults:   evidence,
		EvaluationConfig:    configMap,
		Facts:               facts,
		OverallScore:        utils.PtrFloatOrZero(eval.OverallScore),
		EligibilityStatus:   utils.PtrStringOrEmpty(eval.EligibilityStatus),
		RecommendationLabel: utils.PtrStringOrEmpty(eval.RecommendationLabel),
		JobID:               userJob.JobID,
		UserID:              userJob.UserID.String(),
	}, nil
}
