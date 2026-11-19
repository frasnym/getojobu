package models

// ScrapeURLParams holds the user-supplied URL for scraping.
type ScrapeURLParams struct {
	SourceURL string
}

// UserJobScrapeWorkflowInput is the payload sent to the Python worker's
// UserJobScrapeWorkflow through the concurrency limiter.
type UserJobScrapeWorkflowInput struct {
	JobID     int64  `json:"job_id"`
	Slug      string `json:"slug"`
	SourceURL string `json:"source_url"`
	UserID    string `json:"user_id"`
	OwnerID   string `json:"owner_id"`
}
