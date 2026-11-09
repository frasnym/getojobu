package models

import (
	"strings"
	"time"
	"unicode"
)

// Job is a shared job listing row, returned as an API response.
type Job struct {
	ID             int64     `json:"id"`
	Slug           string    `json:"slug"`
	Status         string    `json:"status"`
	AnalyzeResult  *string   `json:"analyze_result,omitempty"`
	FailureMessage *string   `json:"failure_message,omitempty"`
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
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	UpdatedByType  string    `json:"updated_by_type"`
	UpdatedByID    string    `json:"updated_by_id"`
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
}

// CreateJobResult is the outcome of creating a job listing.
type CreateJobResult struct {
	Job     *Job
	Created bool
}

// BulkCreateJobParams holds parameters for creating many job listings at once.
type BulkCreateJobParams struct {
	Jobs []CreateJobParams `json:"jobs"`
}

// BulkCreateJobResult is the outcome of creating a single job in a bulk create.
type BulkCreateJobResult struct {
	Slug    string `json:"slug"`
	Created bool   `json:"created"`
}

// UpdateJobParams holds parameters for updating a job listing. Slug is taken
// from the request path, not the body.
type UpdateJobParams struct {
	Slug          string `json:"-"`
	Description   string `json:"description"`
	UpdatedByType string `json:"-"`
	UpdatedByID   string `json:"-"`
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
	title = slugify(title)
	if title == "" {
		title = "untitled"
	}
	company = slugify(company)
	if company == "" {
		company = "unknown-company"
	}
	location = slugify(location)
	if location == "" {
		location = "unspecified-location"
	}
	return title + "-" + company + "-" + location
}

// slugify trims, lowercases, kebab-cases (replaces non-alphanumeric runs with
// a single hyphen), and strips leading/trailing hyphens.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteRune('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
