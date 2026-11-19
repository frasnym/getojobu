package models

import "github.com/google/uuid"

// FailedMatchingEvaluation is one evaluation with a failed workflow state.
// EvaluationStatus is NOT NULL; the other status columns are nullable.
type FailedMatchingEvaluation struct {
	ID                uuid.UUID
	Slug              string
	UserID            string
	EvaluationStatus  string
	ReportStatus      *string
	ResumeStatus      *string
	CoverLetterStatus *string
}

// RetryJobsResult counts job analysis workflows re-queued by the sweep.
type RetryJobsResult struct {
	Requeued int `json:"requeued"`
}

// RetryMatchingEvaluationsResult counts re-queued workflows per kind.
type RetryMatchingEvaluationsResult struct {
	Evaluations  int `json:"evaluations"`
	Reports      int `json:"reports"`
	Resumes      int `json:"resumes"`
	CoverLetters int `json:"cover_letters"`
}
