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
	// UserIDs is the list of internal user ids linked to this job in user_jobs.
	UserIDs []string `json:"-"`
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

// JobAnalyzeWorkflowInput is the payload sent to Temporal JobAnalyzeWorkflow.
type JobAnalyzeWorkflowInput struct {
	JobDescription string `json:"job_description"`
	JobID          string `json:"job_id"`
	Slug           string `json:"slug"`
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
