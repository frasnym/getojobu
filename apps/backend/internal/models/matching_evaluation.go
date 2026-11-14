package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// CategoryEvidence links candidate evidence to job requirements.
type CategoryEvidence struct {
	CandidateEvidence string `json:"candidate_evidence"`
	JobRequirement    string `json:"job_requirement"`
	Relationship      string `json:"relationship"` // direct | transferable
}

// CategoryEvaluation represents LLM assessment of a single scoring category.
type CategoryEvaluation struct {
	Category  string             `json:"category"`
	Score     int                `json:"score"`
	Reason    string             `json:"reason"`
	Strengths []string           `json:"strengths,omitempty"`
	Gaps      []string           `json:"gaps,omitempty"`
	Evidence  []CategoryEvidence `json:"evidence,omitempty"`
}

// MandatoryRequirementAssessment represents assessment of a mandatory requirement.
type MandatoryRequirementAssessment struct {
	Requirement string   `json:"requirement"`
	Status      string   `json:"status"` // met | not_met | uncertain
	Reason      string   `json:"reason"`
	Evidence    []string `json:"evidence,omitempty"`
}

// MatchEvaluationOutput is the raw LLM output structure.
type MatchEvaluationOutput struct {
	Evaluations           []CategoryEvaluation             `json:"evaluations"`
	MandatoryRequirements []MandatoryRequirementAssessment `json:"mandatory_requirements"`
	PromptVersion         string                           `json:"prompt_version,omitempty"`
	Provider              *string                          `json:"provider,omitempty"`
	Model                 *string                          `json:"model,omitempty"`
}

// MatchingEvaluation represents the stored matching evaluation record.
type MatchingEvaluation struct {
	ID                      uuid.UUID       `json:"id"`
	UserJobID               uuid.UUID       `json:"user_job_id"`
	EvaluationStatus        string          `json:"evaluation_status"` // new | pending | success | failed
	CandidateProfileData    json.RawMessage `json:"candidate_profile_data,omitempty" swaggertype:"object"`
	JobProfileData          json.RawMessage `json:"job_profile_data,omitempty" swaggertype:"object"`
	ScoringConfigID         uuid.UUID       `json:"scoring_config_id"`
	EvaluationData          json.RawMessage `json:"evaluation_data,omitempty" swaggertype:"object"`
	OverallScore            *float64        `json:"overall_score,omitempty"`
	EligibilityStatus       *string         `json:"eligibility_status,omitempty"`
	RecommendationLabel     *string         `json:"recommendation_label,omitempty"`
	FailureMessage          *string         `json:"failure_message,omitempty"`
	ReportData              json.RawMessage `json:"report_data,omitempty" swaggertype:"object"`
	ReportStatus            *string         `json:"report_status,omitempty"`
	ReportFailureMessage    *string         `json:"report_failure_message,omitempty"`
	ResumeStatus            *string         `json:"resume_status,omitempty"`
	ResumeFailureMessage    *string         `json:"resume_failure_message,omitempty"`
	ResumePDFKey            *string         `json:"resume_pdf_key,omitempty"`
	ResumeJSONKey           *string         `json:"resume_json_key,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

// MatchingEvaluationReportResponse represents the candidate-facing report view.
type MatchingEvaluationReportResponse struct {
	ID                   uuid.UUID       `json:"id"`
	UserJobID            uuid.UUID       `json:"user_job_id"`
	EvaluationStatus     string          `json:"evaluation_status"`
	ReportStatus         *string         `json:"report_status,omitempty"`
	OverallScore         *float64        `json:"overall_score,omitempty"`
	EligibilityStatus    *string         `json:"eligibility_status,omitempty"`
	RecommendationLabel  *string         `json:"recommendation_label,omitempty"`
	ReportData           json.RawMessage `json:"report_data,omitempty" swaggertype:"object"`
	ReportFailureMessage *string         `json:"report_failure_message,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

// DownloadResumeResponse represents the presigned download URL and metadata for a resume.
type DownloadResumeResponse struct {
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
	Filename    string    `json:"filename"`
}

// MatchingEvaluationSummary represents a lightweight summary of an evaluation.
type MatchingEvaluationSummary struct {
	Slug             string     `json:"slug,omitempty"`
	ID               *uuid.UUID `json:"id,omitempty"`
	UserJobID        *uuid.UUID `json:"user_job_id,omitempty"`
	EvaluationStatus string     `json:"evaluation_status,omitempty"`
	CreatedAt        *time.Time `json:"created_at,omitempty"`
	Error            *string    `json:"error,omitempty"`
}

// BulkAnalyzeCompatibilityItem holds input for a single compatibility evaluation in a batch.
type BulkAnalyzeCompatibilityItem struct {
	Slug  string `json:"slug"`
	Force bool   `json:"force"`
}

// BulkGenerateReportItem holds input for a single report generation in a batch.
type BulkGenerateReportItem struct {
	ID    uuid.UUID `json:"id"`
	Force bool      `json:"force"`
}

// BulkGenerateResumeItem holds input for a resume generation batch.
type BulkGenerateResumeItem struct {
	ID    uuid.UUID `json:"id"`
	Force bool      `json:"force"`
}

// UpdateMatchingEvaluationParams contains fields for updating a matching evaluation.
type UpdateMatchingEvaluationParams struct {
	EvaluationStatus    string
	EvaluationData      json.RawMessage `swaggertype:"object"`
	OverallScore        *float64
	EligibilityStatus   *string
	RecommendationLabel *string
	FailureMessage      *string
}

// UpdateMatchingEvaluationReportParams contains fields for updating a report on a matching evaluation.
type UpdateMatchingEvaluationReportParams struct {
	ReportStatus         string          `json:"report_status"`
	ReportData           json.RawMessage `json:"report_data,omitempty" swaggertype:"object"`
	ReportFailureMessage *string         `json:"report_failure_message,omitempty"`
}

// UpdateMatchingEvaluationResumeParams contains fields for updating resume generation.
type UpdateMatchingEvaluationResumeParams struct {
	ResumeStatus         string
	ResumeFailureMessage *string
	ResumePDFKey         *string
	ResumeJSONKey        *string
}

// ValidateEvaluationOutput validates LLM output against active scoring categories.
func ValidateEvaluationOutput(
	configCategories []CategoryConfig,
	output MatchEvaluationOutput,
) error {
	if len(configCategories) == 0 {
		return errors.New("no scoring configuration categories provided")
	}

	evalMap := make(map[string]CategoryEvaluation)
	for _, ev := range output.Evaluations {
		catName := strings.TrimSpace(ev.Category)
		if catName == "" {
			return errors.New("evaluation category name cannot be empty")
		}
		if _, exists := evalMap[catName]; exists {
			return fmt.Errorf("duplicate evaluation for category: %s", catName)
		}
		if ev.Score < 0 || ev.Score > 100 {
			return fmt.Errorf("score must be between 0 and 100 for category %s, got %d", catName, ev.Score)
		}
		evalMap[catName] = ev
	}

	for _, cfg := range configCategories {
		if _, exists := evalMap[cfg.Name]; !exists {
			return fmt.Errorf("missing evaluation for category: %s", cfg.Name)
		}
	}

	if len(evalMap) != len(configCategories) {
		return fmt.Errorf("unexpected evaluation category: expected %d categories, got %d", len(configCategories), len(evalMap))
	}

	for _, req := range output.MandatoryRequirements {
		status := constants.RequirementStatus(req.Status)
		switch status {
		case constants.RequirementStatusMet,
			constants.RequirementStatusNotMet,
			constants.RequirementStatusUncertain:
			// valid
		default:
			return fmt.Errorf("invalid requirement status %q for %s (must be met, not_met, or uncertain)", req.Status, req.Requirement)
		}
	}

	return nil
}
