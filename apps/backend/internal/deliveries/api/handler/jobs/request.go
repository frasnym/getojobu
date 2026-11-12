package jobs

import (
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

type createJobRequest struct {
	Source         string  `json:"source"`
	ExternalID     *string `json:"external_id,omitempty"`
	SourceURL      *string `json:"source_url,omitempty"`
	ApplyURL       *string `json:"apply_url,omitempty"`
	Title          string  `json:"title"`
	Company        string  `json:"company"`
	Location       string  `json:"location"`
	Description    *string `json:"description,omitempty"`
	PostedDate     string  `json:"posted_date,omitempty"`
	IsRemote       *bool   `json:"is_remote,omitempty"`
	VisaRelocation *bool   `json:"visa_relocation,omitempty"`
	// UserIDs is the list of internal user ids linked to this job.
	// Required for API key callers. Clerk callers must not send it (it is
	// resolved from the session token); sending it is a 400 error.
	UserIDs []string `json:"user_ids,omitempty"`
}

type bulkCreateJobRequest struct {
	Jobs []createJobRequest `json:"jobs"`
}

// updateJobRequest is the PATCH /v1/jobs/{slug} body for status transitions.
// All fields are optional pointers; "status" must be present and non-empty.
type updateJobRequest struct {
	Status         *string `json:"status,omitempty"`
	AnalyzeResult  *string `json:"analyze_result,omitempty"`
	FailureMessage *string `json:"failure_message,omitempty"`
}

// bulkQueueAnalyzeItemRequest is one item in the POST /v1/jobs/bulk-queue-analyze request.
type bulkQueueAnalyzeItemRequest struct {
	Slug  string `json:"slug"`
	Force bool   `json:"force"`
}

// bulkQueueAnalyzeRequest is the array payload for POST /v1/jobs/bulk-queue-analyze.
type bulkQueueAnalyzeRequest []bulkQueueAnalyzeItemRequest

// bulkQueueAnalyzeResponse is the response body for POST /v1/jobs/bulk-queue-analyze.
type bulkQueueAnalyzeResponse struct {
	WorkflowIDs []string `json:"workflow_ids"`
}

// Validate checks the fields required for every job listing: source, title,
// company, and location. It also parses posted_date (YYYY-MM-DD)
// into a time.Time, returning the zero value when empty.
func (j createJobRequest) Validate() (time.Time, error) {
	if j.Source == "" || j.Title == "" || j.Company == "" || j.Location == "" {
		return time.Time{}, httpx.Envelope(httpx.CodeInvalidRequest, "source, title, company, and location are required", "")
	}

	if j.PostedDate == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse("2006-01-02", j.PostedDate)
	if err != nil {
		return time.Time{}, httpx.Envelope(httpx.CodeInvalidRequest, "posted_date must be in YYYY-MM-DD format", "")
	}
	return t, nil
}
