package models

import (
	"encoding/json"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

// Job is a shared job listing row, returned as an API response.
type Job struct {
	ID             int64           `json:"id"`
	Slug           string          `json:"slug"`
	Status         string          `json:"status"`
	AnalyzeResult  json.RawMessage `json:"analyze_result,omitempty" swaggertype:"object"`
	FailureMessage *string         `json:"failure_message,omitempty"`
	Source         string          `json:"source"`
	ExternalID     *string         `json:"external_id,omitempty"`
	SourceURL      *string         `json:"source_url,omitempty"`
	ApplyURL       *string         `json:"apply_url,omitempty"`
	Title          string          `json:"title"`
	Company        string          `json:"company"`
	Location       string          `json:"location"`
	Description    *string         `json:"description,omitempty"`
	PostedDate     time.Time       `json:"posted_date,omitempty"`
	IsRemote       *bool           `json:"is_remote,omitempty"`
	VisaRelocation *bool           `json:"visa_relocation,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	UpdatedByType  string          `json:"updated_by_type"`
	UpdatedByID    string          `json:"updated_by_id"`
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
