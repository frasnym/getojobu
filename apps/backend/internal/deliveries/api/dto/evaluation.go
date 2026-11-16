package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

// MatchingEvaluationResponse is a stored matching evaluation without its job.
// Worker-facing endpoints use it so their payload keeps every key it always had.
type MatchingEvaluationResponse struct {
	ID                   uuid.UUID       `json:"id"`
	UserJobID            uuid.UUID       `json:"user_job_id"`
	EvaluationStatus     string          `json:"evaluation_status"`
	EvaluationData       json.RawMessage `json:"evaluation_data,omitempty" swaggertype:"object"`
	OverallScore         *float64        `json:"overall_score,omitempty"`
	EligibilityStatus    *string         `json:"eligibility_status,omitempty"`
	RecommendationLabel  *string         `json:"recommendation_label,omitempty"`
	FailureMessage       *string         `json:"failure_message,omitempty"`
	ReportData           json.RawMessage `json:"report_data,omitempty" swaggertype:"object"`
	ReportStatus         *string         `json:"report_status,omitempty"`
	ReportFailureMessage *string         `json:"report_failure_message,omitempty"`
	ResumeStatus         *string         `json:"resume_status,omitempty"`
	ResumeFailureMessage *string         `json:"resume_failure_message,omitempty"`
	ResumePDFKey         *string         `json:"resume_pdf_key,omitempty"`
	ResumeJSONKey        *string         `json:"resume_json_key,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

// EvaluationResponse is a matching evaluation together with the job it
// evaluates, as returned on the candidate-facing endpoint.
type EvaluationResponse struct {
	MatchingEvaluationResponse
	Job JobResponse `json:"job"`
}

// NewMatchingEvaluationResponse maps an evaluation to its job-less API shape.
func NewMatchingEvaluationResponse(eval *models.MatchingEvaluation) MatchingEvaluationResponse {
	if eval == nil {
		return MatchingEvaluationResponse{}
	}

	return MatchingEvaluationResponse{
		ID:                   eval.ID,
		UserJobID:            eval.UserJobID,
		EvaluationStatus:     eval.EvaluationStatus,
		EvaluationData:       eval.EvaluationData,
		OverallScore:         eval.OverallScore,
		EligibilityStatus:    eval.EligibilityStatus,
		RecommendationLabel:  eval.RecommendationLabel,
		FailureMessage:       eval.FailureMessage,
		ReportData:           eval.ReportData,
		ReportStatus:         eval.ReportStatus,
		ReportFailureMessage: eval.ReportFailureMessage,
		ResumeStatus:         eval.ResumeStatus,
		ResumeFailureMessage: eval.ResumeFailureMessage,
		ResumePDFKey:         eval.ResumePDFKey,
		ResumeJSONKey:        eval.ResumeJSONKey,
		CreatedAt:            eval.CreatedAt,
		UpdatedAt:            eval.UpdatedAt,
	}
}

// NewEvaluationResponse maps an evaluation plus its job to the candidate-facing
// API shape.
func NewEvaluationResponse(eval *models.MatchingEvaluation, job *models.Job) EvaluationResponse {
	return EvaluationResponse{
		MatchingEvaluationResponse: NewMatchingEvaluationResponse(eval),
		Job:                        NewJobResponse(job, nil),
	}
}
