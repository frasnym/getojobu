package models

// ScrapeJobsRequest is the body for POST /v1/jobs/scrape.
type ScrapeJobsRequest struct {
	UserIDs []string `json:"user_ids,omitempty"`
}

// ScrapeJobsResponse is the 202 Accepted body.
type ScrapeJobsResponse struct {
	WorkflowIDs []string `json:"workflow_ids"`
}

// ScrapeJobsParams holds the input for starting job scrape workflows.
type ScrapeJobsParams struct {
	UserIDs []string
}

// NormalizedScrapeTarget holds a deduplicated (board, keyword, location) query
// associated with all users whose preferences matched it.
type NormalizedScrapeTarget struct {
	Board    string   `json:"board"`
	Keyword  string   `json:"keyword"`
	Location string   `json:"location"`
	UserIDs  []string `json:"user_ids"`
}

// JobScrapeWorkflowInput is the payload sent to the Python worker's
// JobScrapeWorkflow.
type JobScrapeWorkflowInput struct {
	Board      string   `json:"board"`
	Keyword    string   `json:"keyword"`
	Location   string   `json:"location"`
	UserIDs    []string `json:"user_ids"`
	SearchID   string   `json:"search_id"`
	WorkflowID string   `json:"workflow_id"`
}
