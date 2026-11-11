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
	// UserID is the internal user id that owns this job listing.
	// Required for API key callers. Clerk callers must not send it (it is
	// resolved from the token); sending it is a 400 error.
	UserID string `json:"user_id,omitempty"`
}

type bulkCreateJobRequest struct {
	Jobs []createJobRequest `json:"jobs"`
}

type updateJobDescriptionRequest struct {
	Description string `json:"description"`
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
