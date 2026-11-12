package models

// ScrapeSearch holds one keyword × location search request.
type ScrapeSearch struct {
	Keywords  []string `json:"keywords"`
	Locations []string `json:"locations"`
}

// ScrapeJobsRequest is the body for POST /v1/jobs/scrape.
type ScrapeJobsRequest struct {
	Searches []ScrapeSearch `json:"searches"`
}

// ScrapeJobsResponse is the 202 Accepted body.
type ScrapeJobsResponse struct {
	WorkflowID string `json:"workflow_id"`
}

// ScrapeJobsParams holds the input for starting a JobScrapeWorkflow via the
// Temporal repository.
type ScrapeJobsParams struct {
	UserID     string
	SearchID   string
	Searches   []ScrapeSearch
	WorkflowID string
}

// JobScrapeWorkflowInput is the payload sent to the Python worker's
// JobScrapeWorkflow.
type JobScrapeWorkflowInput struct {
	UserID     string         `json:"user_id"`
	SearchID   string         `json:"search_id"`
	Searches   []ScrapeSearch `json:"searches"`
	WorkflowID string         `json:"workflow_id"`
}
