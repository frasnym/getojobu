package matching

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

// GetEvaluation returns the stored input for MatchJobWorkflow. The profile
// snapshots come from the evaluation row so retries use the same data as the
// original queue, and the config is resolved by the row's config ID.
func (s *matchingService) GetEvaluation(ctx context.Context, id uuid.UUID) (*models.GetEvaluationResponse, error) {
	eval, err := s.evalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	config, err := s.scoringConfigsRepo.GetByID(ctx, eval.ScoringConfigID)
	if err != nil {
		return nil, err
	}

	var candidate, job map[string]interface{}
	_ = json.Unmarshal(eval.CandidateProfileData, &candidate)
	_ = json.Unmarshal(eval.JobProfileData, &job)
	if candidate == nil {
		candidate = map[string]interface{}{}
	}
	if job == nil {
		job = map[string]interface{}{}
	}

	configBytes, _ := json.Marshal(config)
	var configMap map[string]interface{}
	_ = json.Unmarshal(utils.StripJSONKeys(configBytes, "id", "version", "is_active", "created_at", "updated_at"), &configMap)
	if configMap == nil {
		configMap = map[string]interface{}{}
	}

	return &models.GetEvaluationResponse{
		EvaluationStatus: eval.EvaluationStatus,
		CandidateProfile: candidate,
		JobProfile:       job,
		EvaluationConfig: configMap,
	}, nil
}
