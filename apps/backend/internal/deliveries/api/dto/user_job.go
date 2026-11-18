package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

// UserJobEvaluationResponse is a lightweight summary of the latest matching
// evaluation for a user-job link.
type UserJobEvaluationResponse struct {
	ID                  uuid.UUID `json:"id"`
	EvaluationStatus    string    `json:"evaluation_status"`
	EligibilityStatus   *string   `json:"eligibility_status,omitempty"`
	RecommendationLabel *string   `json:"recommendation_label,omitempty"`
	ReportStatus        *string   `json:"report_status,omitempty"`
	ResumeStatus        *string   `json:"resume_status,omitempty"`
	CoverLetterStatus   *string   `json:"cover_letter_status,omitempty"`
}

// UserJobResponse is a user's link to a shared job listing. The root identity is
// the user_jobs row; the listing lives inside Job.
type UserJobResponse struct {
	ID         uuid.UUID                  `json:"id"`
	IsHidden   bool                       `json:"is_hidden"`
	CreatedAt  time.Time                  `json:"created_at"`
	UpdatedAt  time.Time                  `json:"updated_at"`
	Job        JobResponse                `json:"job"`
	Evaluation *UserJobEvaluationResponse `json:"evaluation,omitempty"`
}

// NewUserJobResponse maps a user-job link plus its listing and evaluation statuses to the API shape.
func NewUserJobResponse(row *models.UserJobWithJob) UserJobResponse {
	if row == nil || row.UserJob == nil {
		return UserJobResponse{}
	}

	var evalResp *UserJobEvaluationResponse
	if row.Evaluation != nil {
		evalResp = &UserJobEvaluationResponse{
			ID:                  row.Evaluation.ID,
			EvaluationStatus:    row.Evaluation.EvaluationStatus,
			EligibilityStatus:   row.Evaluation.EligibilityStatus,
			RecommendationLabel: row.Evaluation.RecommendationLabel,
			ReportStatus:        row.Evaluation.ReportStatus,
			ResumeStatus:        row.Evaluation.ResumeStatus,
			CoverLetterStatus:   row.Evaluation.CoverLetterStatus,
		}
	}

	return UserJobResponse{
		ID:         row.UserJob.ID,
		IsHidden:   row.UserJob.IsHidden,
		CreatedAt:  row.UserJob.CreatedAt,
		UpdatedAt:  row.UserJob.UpdatedAt,
		Job:        NewJobResponse(row.Job, nil),
		Evaluation: evalResp,
	}
}
