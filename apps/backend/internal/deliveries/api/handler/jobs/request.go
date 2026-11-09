package jobs

import (
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

type createJobRequest struct {
	Slug           string  `json:"slug"`
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
}

type bulkCreateJobRequest struct {
	Jobs []createJobRequest `json:"jobs"`
}

type updateJobDescriptionRequest struct {
	Description string `json:"description"`
}

// Validate checks the fields required for every job listing: slug, source,
// title, company, and location.
func (j createJobRequest) Validate() error {
	if j.Slug == "" || j.Source == "" || j.Title == "" || j.Company == "" || j.Location == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "slug, source, title, company, and location are required", "")
	}
	return nil
}
