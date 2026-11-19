package matching

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

// GetEvaluation returns the stored input for the evaluation workflows. The
// snapshots come from the evaluation row so retries use the same data as the
// original queue; the resume facts and config are composed by the same helper
// the queue step uses.
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

	resumeInput, err := s.buildResumeInput(ctx, eval, userJob, user)
	if err != nil {
		return nil, err
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
		CandidateProfile:    resumeInput.CandidateProfile,
		JobProfile:          resumeInput.JobProfile,
		EvaluationResults:   resumeInput.EvaluationResults,
		EvaluationConfig:    configMap,
		Facts:               resumeInput.Facts,
		OverallScore:        resumeInput.OverallScore,
		EligibilityStatus:   resumeInput.EligibilityStatus,
		RecommendationLabel: resumeInput.RecommendationLabel,
		JobID:               resumeInput.JobID,
		UserID:              resumeInput.UserID,
	}, nil
}
