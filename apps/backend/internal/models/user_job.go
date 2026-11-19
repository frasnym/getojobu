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
	ID        uuid.UUID
	UserID    uuid.UUID
	JobID     int64
	IsHidden  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserJobWithJob is an internal read model coupling a user_jobs link to its
// shared job listing and optional latest matching evaluation.
type UserJobWithJob struct {
	UserJob    *UserJob
	Job        *Job
	Evaluation *MatchingEvaluation
}
