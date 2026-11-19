package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

// Job is a shared job listing row.
type Job struct {
	ID             int64
	Slug           string
	Status         string
	AnalyzeResult  json.RawMessage
	FailureMessage *string
	Source         string
	ExternalID     *string
	SourceURL      *string
	ApplyURL       *string
	Title          string
	Company        string
	Location       string
	Description    *string
	PostedDate     time.Time
	IsRemote       *bool
	VisaRelocation *bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	UpdatedByType  string
	UpdatedByID    string
	OwnerID        *uuid.UUID
}

// UserJobFilter holds optional server-side filters when listing a user's jobs.
// An empty field means "no constraint"; Usecase/handler pass only the values
// the caller explicitly requested.
type UserJobFilter struct {
	// Status restricts to a single job status. Empty or "all" means no filter.
	Status string `json:"status,omitempty"`
	// RemoteOnly restricts to jobs flagged is_remote=true.
	RemoteOnly bool `json:"remote,omitempty"`
	// HideFilter restricts by the user's hidden flag: "" or "all" (no filter),
	// "visible" (is_hidden=false) or "hidden" (is_hidden=true).
	HideFilter string `json:"hide,omitempty"`
	// EvaluationStatus restricts by matching_evaluations.evaluation_status.
	EvaluationStatus string `json:"evaluation_status,omitempty"`
	// EligibilityStatus restricts by matching_evaluations.eligibility_status.
	EligibilityStatus string `json:"eligibility_status,omitempty"`
	// RecommendationLabel restricts by matching_evaluations.recommendation_label.
	RecommendationLabel string `json:"recommendation_label,omitempty"`
	// ReportStatus restricts by matching_evaluations.report_status.
	ReportStatus string `json:"report_status,omitempty"`
	// ResumeStatus restricts by matching_evaluations.resume_status.
	ResumeStatus string `json:"resume_status,omitempty"`
}

// JobWithOwnership is a shared job listing plus the requesting user's link to
// it. It is an internal read model: repositories fill it, use cases pass it
// through, and handlers map it to an API response.
type JobWithOwnership struct {
	Job *Job
	// IsOwned reports whether the requesting user has a user_jobs row for Job.
	IsOwned bool
	// UserJobID is the user_jobs row id, set only when IsOwned is true.
	UserJobID *uuid.UUID
}

// CreateJobParams holds parameters for creating a single job listing.
type CreateJobParams struct {
	Slug           string    `json:"slug"`
	Source         string    `json:"source"`
	ExternalID     *string   `json:"external_id,omitempty"`
	SourceURL      *string   `json:"source_url,omitempty"`
	ApplyURL       *string   `json:"apply_url,omitempty"`
	Title          string    `json:"title"`
	Company        string    `json:"company"`
	Location       string    `json:"location"`
	Description    *string   `json:"description,omitempty"`
	PostedDate     time.Time `json:"posted_date,omitempty"`
	IsRemote       *bool     `json:"is_remote,omitempty"`
	VisaRelocation *bool     `json:"visa_relocation,omitempty"`
	// Audit attribution for the actor creating the listing. Columns are NOT NULL
	// with no server default; the handler always supplies them (defaults to
	// ("system", "system") via actorTypeID() in the repository when empty).
	UpdatedByType string `json:"-"`
	UpdatedByID   string `json:"-"`
	// OwnerID marks a private, user-created listing. NULL means shared.
	OwnerID *uuid.UUID `json:"-"`
	// Status lets callers insert a listing at a specific status. Empty uses the
	// database default of "new".
	Status string `json:"-"`
	// UserIDs is the list of internal user ids linked to this job in user_jobs.
	UserIDs []string `json:"-"`
}

// UpdateJobParams carries the fields for a status transition. The extended
// fields are applied only when the current status is scrape_pending.
type UpdateJobParams struct {
	Slug           string
	Status         string
	AnalyzeResult  *string
	FailureMessage *string
	NewSlug        *string
	Title          *string
	Company        *string
	Location       *string
	Description    *string
	SourceURL      *string
	ApplyURL       *string
	PostedDate     *time.Time
	IsRemote       *bool
	VisaRelocation *bool
	UpdatedByType  string
	UpdatedByID    string
}

// CreateManualJobParams holds user-supplied fields for a manually inserted job.
type CreateManualJobParams struct {
	Title          string
	Company        string
	Location       string
	Description    *string
	SourceURL      *string
	ApplyURL       *string
	PostedDate     time.Time
	IsRemote       *bool
	VisaRelocation *bool
}

// CreateJobResult is the outcome of creating a job listing.
type CreateJobResult struct {
	Job             *Job
	Created         bool
	AddedToUserList bool
}

// BulkCreateJobParams holds parameters for creating many job listings at once.
type BulkCreateJobParams struct {
	Jobs []CreateJobParams `json:"jobs"`
}

// BulkCreateJobResult is the outcome of creating a single job in a bulk create.
type BulkCreateJobResult struct {
	Slug            string `json:"slug"`
	Created         bool   `json:"created"`
	AddedToUserList bool   `json:"added_to_user_list"`
}

// QueueAnalysisItem holds the parameters for queuing analysis for a single job.
type QueueAnalysisItem struct {
	Slug  string `json:"slug"`
	Force bool   `json:"force"`
}

// QueueAnalysisFailure is one job skipped during a bulk queue analyze request.
type QueueAnalysisFailure struct {
	Slug  string `json:"slug"`
	Error string `json:"error"`
}

// QueueAnalysisBulkResult holds the per-item outcomes of a bulk queue analyze request.
type QueueAnalysisBulkResult struct {
	WorkflowIDs []string               `json:"workflow_ids"`
	Failed      []QueueAnalysisFailure `json:"failed,omitempty"`
}

// JobAnalyzeWorkflowInput is the payload sent to Temporal JobAnalyzeWorkflow.
type JobAnalyzeWorkflowInput struct {
	Slug string `json:"slug"`
}

// GetJobParams holds parameters for getting a job listing by slug. Slug is
// taken from the request path, not the body.
type GetJobParams struct {
	Slug string `json:"-"`
}

// ListJobsParams holds parameters for listing job listings.
type ListJobsParams struct {
	Limit   int    `json:"limit,omitempty"`
	Company string `json:"company,omitempty"`
}

// SearchJobsParams holds parameters for searching job listings by company.
type SearchJobsParams struct {
	Company string `json:"company"`
	Limit   int    `json:"limit,omitempty"`
}

// BuildJobSlug normalizes title/company/location into a deterministic slug:
// {normalized-title}-{normalized-company}-{normalized-location}
// Missing components fall back to "unknown-company" / "unspecified-location".
func BuildJobSlug(title, company, location string) string {
	title = utils.Slugify(title)
	if title == "" {
		title = "untitled"
	}
	company = utils.Slugify(company)
	if company == "" {
		company = "unknown-company"
	}
	location = utils.Slugify(location)
	if location == "" {
		location = "unspecified-location"
	}
	return title + "-" + company + "-" + location
}

// BuildManualJobSlug prefixes the standard slug with "m-{first8 of userID}-"
// to namespace manual jobs away from scraped jobs and from other users.
func BuildManualJobSlug(userID, title, company, location string) string {
	prefix := "m-"
	if len(userID) >= 8 {
		prefix += userID[:8] + "-"
	} else if userID != "" {
		prefix += userID + "-"
	}
	return prefix + BuildJobSlug(title, company, location)
}
