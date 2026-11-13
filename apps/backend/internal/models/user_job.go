package models

import (
	"time"

	"github.com/google/uuid"
)

// UserJobLink links a user to a job in user_jobs.
type UserJobLink struct {
	UserID string
	JobID  int64
}

// UserJob represents a row in user_jobs linking a user to a job.
type UserJob struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	JobID     int64     `json:"job_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
