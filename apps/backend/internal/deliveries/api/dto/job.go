// Package dto holds API response shapes. Domain structs in internal/models are
// never serialized directly; handlers map them through the constructors here.
package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

// JobResponse is a shared job listing as returned by the API. It carries no
// per-user state except the caller-relative IsOwned flag and, on the
// single-job fetch only, the caller's UserJob link.
type JobResponse struct {
	ID             int64           `json:"id"`
	Slug           string          `json:"slug"`
	Status         string          `json:"status"`
	AnalyzeResult  json.RawMessage `json:"analyze_result,omitempty" swaggertype:"object"`
	FailureMessage *string         `json:"failure_message,omitempty"`
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

	// IsOwned reports whether the requesting user already has this listing in
	// their list. Omitted for callers with no user context (API keys).
	IsOwned *bool `json:"is_owned,omitempty"`

	// UserJob is the caller's link to this listing. Populated only by the
	// single-job fetch, and only when the caller owns the listing.
	UserJob *UserJobRef `json:"user_job,omitempty"`
}

// UserJobRef is the minimal reference to a user_jobs row.
type UserJobRef struct {
	ID uuid.UUID `json:"id"`
}

// NewJobResponse maps a job listing to its API shape. isOwned is nil for
// callers with no user context, which omits the flag from the payload.
func NewJobResponse(job *models.Job, isOwned *bool) JobResponse {
	if job == nil {
		return JobResponse{}
	}

	return JobResponse{
		ID:             job.ID,
		Slug:           job.Slug,
		Status:         job.Status,
		AnalyzeResult:  job.AnalyzeResult,
		FailureMessage: job.FailureMessage,
		ExternalID:     job.ExternalID,
		SourceURL:      job.SourceURL,
		ApplyURL:       job.ApplyURL,
		Title:          job.Title,
		Company:        job.Company,
		Location:       job.Location,
		Description:    job.Description,
		PostedDate:     job.PostedDate,
		IsRemote:       job.IsRemote,
		VisaRelocation: job.VisaRelocation,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
		IsOwned:        isOwned,
	}
}

// NewJobDetailResponse maps the single-job fetch, where the caller's link is
// exposed so the workspace page can reach the evaluation endpoint.
func NewJobDetailResponse(job *models.Job, isOwned *bool, userJobID *uuid.UUID) JobResponse {
	if job == nil {
		return JobResponse{}
	}
	res := NewJobResponse(job, isOwned)
	if userJobID == nil {
		return res
	}

	owned := true
	res.IsOwned = &owned
	res.UserJob = &UserJobRef{ID: *userJobID}
	return res
}

// CreateJobResponse is the outcome of creating a single job listing.
type CreateJobResponse struct {
	Job             JobResponse `json:"job"`
	Created         bool        `json:"created"`
	AddedToUserList bool        `json:"added_to_user_list"`
}

// NewCreateJobResponse maps the create result to its API shape.
func NewCreateJobResponse(result *models.CreateJobResult) CreateJobResponse {
	if result == nil {
		return CreateJobResponse{}
	}

	return CreateJobResponse{
		Job:             NewJobResponse(result.Job, nil),
		Created:         result.Created,
		AddedToUserList: result.AddedToUserList,
	}
}
