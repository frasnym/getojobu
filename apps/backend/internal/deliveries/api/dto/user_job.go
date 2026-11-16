package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

// UserJobResponse is a user's link to a shared job listing. The root identity is
// the user_jobs row; the listing lives inside Job.
type UserJobResponse struct {
	ID        uuid.UUID   `json:"id"`
	IsHidden  bool        `json:"is_hidden"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	Job       JobResponse `json:"job"`
}

// NewUserJobResponse maps a user-job link plus its listing to the API shape.
func NewUserJobResponse(userJob *models.UserJob, job *models.Job) UserJobResponse {
	if userJob == nil {
		return UserJobResponse{}
	}

	return UserJobResponse{
		ID:        userJob.ID,
		IsHidden:  userJob.IsHidden,
		CreatedAt: userJob.CreatedAt,
		UpdatedAt: userJob.UpdatedAt,
		Job:       NewJobResponse(job, nil),
	}
}
