package models

// LimiterJobType defines the allowed job types for ConcurrencyLimiter work items.
type LimiterJobType string

const (
	LimiterJobTypeScrape  LimiterJobType = "scrape"
	LimiterJobTypeAnalyze LimiterJobType = "analyze"
	LimiterJobTypeMatch   LimiterJobType = "match"
	LimiterJobTypeReport  LimiterJobType = "report"
	LimiterJobTypeResume  LimiterJobType = "resume"
)

// LimiterWorkItem represents a work item enqueued into ConcurrencyLimiter.
type LimiterWorkItem struct {
	Params  interface{}    `json:"params"`
	ItemID  string         `json:"item_id"`
	JobType LimiterJobType `json:"job_type"`
}
