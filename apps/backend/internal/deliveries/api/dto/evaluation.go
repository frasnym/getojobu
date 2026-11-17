package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

// CoverLetterContentResponse is the generated letter. Model and prompt version
// stay stored with the letter but are not exposed by the public API.
type CoverLetterContentResponse struct {
	Text string `json:"text"`
}

// MatchingEvaluationResponse is a stored matching evaluation without its job.
// Worker-facing endpoints use it so their payload keeps every key it always had.
type MatchingEvaluationResponse struct {
	ID                        uuid.UUID                   `json:"id"`
	UserJobID                 uuid.UUID                   `json:"user_job_id"`
	EvaluationStatus          string                      `json:"evaluation_status"`
	EvaluationData            json.RawMessage             `json:"evaluation_data,omitempty" swaggertype:"object"`
	OverallScore              *float64                    `json:"overall_score,omitempty"`
	EligibilityStatus         *string                     `json:"eligibility_status,omitempty"`
	RecommendationLabel       *string                     `json:"recommendation_label,omitempty"`
	FailureMessage            *string                     `json:"failure_message,omitempty"`
	ReportData                json.RawMessage             `json:"report_data,omitempty" swaggertype:"object"`
	ReportStatus              *string                     `json:"report_status,omitempty"`
	ReportFailureMessage      *string                     `json:"report_failure_message,omitempty"`
	ResumeStatus              *string                     `json:"resume_status,omitempty"`
	ResumeFailureMessage      *string                     `json:"resume_failure_message,omitempty"`
	ResumePDFKey              *string                     `json:"resume_pdf_key,omitempty"`
	ResumeJSONKey             *string                     `json:"resume_json_key,omitempty"`
	CoverLetterStatus         *string                     `json:"cover_letter_status,omitempty"`
	CoverLetterContent        *CoverLetterContentResponse `json:"cover_letter_content,omitempty" swaggertype:"object"`
	CoverLetterFailureMessage *string                     `json:"cover_letter_failure_message,omitempty"`
	CoverLetterFileName       string                      `json:"cover_letter_file_name"`
	CreatedAt                 time.Time                   `json:"created_at"`
	UpdatedAt                 time.Time                   `json:"updated_at"`
}

// EvaluationResponse is a matching evaluation together with the job it
// evaluates, as returned on the candidate-facing endpoint. The evaluation is
// nested under its own "evaluation" key (mirroring "job"), so it is not merged
// with the user-jobs data at the root.
type EvaluationResponse struct {
	ID         uuid.UUID                   `json:"id"`
	UserJobID  uuid.UUID                   `json:"user_job_id"`
	Job        JobResponse                 `json:"job"`
	Evaluation *MatchingEvaluationResponse `json:"evaluation"`
}

// NewMatchingEvaluationResponse maps an evaluation to its job-less API shape.
func NewMatchingEvaluationResponse(eval *models.MatchingEvaluation) MatchingEvaluationResponse {
	if eval == nil {
		return MatchingEvaluationResponse{}
	}

	var coverLetterContent *CoverLetterContentResponse
	if len(eval.CoverLetterContent) > 0 {
		var parsed CoverLetterContentResponse
		if err := json.Unmarshal(eval.CoverLetterContent, &parsed); err == nil {
			coverLetterContent = &parsed
		}
	}

	return MatchingEvaluationResponse{
		ID:                        eval.ID,
		UserJobID:                 eval.UserJobID,
		EvaluationStatus:          eval.EvaluationStatus,
		EvaluationData:            eval.EvaluationData,
		OverallScore:              eval.OverallScore,
		EligibilityStatus:         eval.EligibilityStatus,
		RecommendationLabel:       eval.RecommendationLabel,
		FailureMessage:            eval.FailureMessage,
		ReportData:                eval.ReportData,
		ReportStatus:              eval.ReportStatus,
		ReportFailureMessage:      eval.ReportFailureMessage,
		ResumeStatus:              eval.ResumeStatus,
		ResumeFailureMessage:      eval.ResumeFailureMessage,
		ResumePDFKey:              eval.ResumePDFKey,
		ResumeJSONKey:             eval.ResumeJSONKey,
		CoverLetterStatus:         eval.CoverLetterStatus,
		CoverLetterContent:        coverLetterContent,
		CoverLetterFailureMessage: eval.CoverLetterFailureMessage,
		CoverLetterFileName:       eval.CoverLetterFileName,
		CreatedAt:                 eval.CreatedAt,
		UpdatedAt:                 eval.UpdatedAt,
	}
}

// NewEvaluationResponse maps an evaluation plus its job to the candidate-facing
// API shape.
func NewEvaluationResponse(eval *models.MatchingEvaluation, job *models.Job) EvaluationResponse {
	evalResp := NewMatchingEvaluationResponse(eval)
	return EvaluationResponse{
		ID:         eval.ID,
		UserJobID:  eval.UserJobID,
		Job:        NewJobResponse(job, nil),
		Evaluation: &evalResp,
	}
}
