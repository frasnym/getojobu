package constants

import "fmt"

// JobStatus is the typed status of a job listing.
type JobStatus string

// Job status constants.
const (
	StatusNew            JobStatus = "new"
	StatusQueued         JobStatus = "queued"
	StatusQueuedRetry    JobStatus = "queued_retry"
	StatusAnalyzePending JobStatus = "analyze_pending"
	StatusAnalyzed       JobStatus = "analyzed"
	StatusFailed         JobStatus = "failed"
	StatusScrapeNew      JobStatus = "scrape_new"
	StatusScrapePending  JobStatus = "scrape_pending"
	StatusScrapeFailed   JobStatus = "scrape_failed"
)

// ValidateJobStatusTransition returns an error when transitioning from current
// to next is not permitted. Allowed paths:
//
//	""              → queued
//	new             → queued
//	queued          → analyze_pending
//	queued          → queued_retry    (force retry)
//	queued_retry    → analyze_pending
//	analyze_pending → queued_retry    (stale retry)
//	analyze_pending → analyzed
//	analyze_pending → failed
//	failed          → queued_retry    (retry)
//	scrape_new      → scrape_pending  (worker starts URL scrape)
//	scrape_new      → scrape_failed   (dispatch/setup failure before scrape)
//	scrape_pending  → new             (scrape succeeded)
//	scrape_pending  → scrape_failed   (scrape failed)
//	scrape_failed   → scrape_pending  (retry scrape)
//
// Everything else is rejected, including any transition out of "analyzed".
func ValidateJobStatusTransition(current, next JobStatus) error {
	// Normalize a blank current status to "new" so callers don't have to.
	if current == "" {
		current = StatusNew
	}

	switch current {
	case StatusNew:
		if next == StatusQueued {
			return nil
		}
		return fmt.Errorf(
			"transition from %q to %q is not allowed: job must move to %q first",
			current, next, StatusQueued,
		)

	case StatusScrapeNew:
		if next == StatusScrapePending || next == StatusScrapeFailed {
			return nil
		}
		return fmt.Errorf(
			"transition from %q to %q is not allowed: expected %q or %q",
			current, next, StatusScrapePending, StatusScrapeFailed,
		)

	case StatusQueued:
		if next == StatusAnalyzePending || next == StatusQueuedRetry {
			return nil
		}
		return fmt.Errorf(
			"transition from %q to %q is not allowed: expected %q or %q",
			current, next, StatusAnalyzePending, StatusQueuedRetry,
		)

	case StatusQueuedRetry:
		if next == StatusAnalyzePending {
			return nil
		}
		return fmt.Errorf(
			"transition from %q to %q is not allowed: expected %q",
			current, next, StatusAnalyzePending,
		)

	case StatusAnalyzePending:
		if next == StatusQueuedRetry || next == StatusAnalyzed || next == StatusFailed {
			return nil
		}
		return fmt.Errorf(
			"transition from %q to %q is not allowed: expected %q, %q or %q",
			current, next, StatusQueuedRetry, StatusAnalyzed, StatusFailed,
		)

	case StatusAnalyzed:
		return fmt.Errorf(
			"transition from %q to %q is not allowed: job is already finalized",
			current, next,
		)

	case StatusFailed:
		if next == StatusQueuedRetry {
			return nil
		}
		return fmt.Errorf(
			"transition from %q to %q is not allowed: failed jobs may only be retried via %q",
			current, next, StatusQueuedRetry,
		)

	case StatusScrapePending:
		if next == StatusNew || next == StatusScrapeFailed {
			return nil
		}
		return fmt.Errorf(
			"transition from %q to %q is not allowed: expected %q or %q",
			current, next, StatusNew, StatusScrapeFailed,
		)

	case StatusScrapeFailed:
		if next == StatusScrapePending {
			return nil
		}
		return fmt.Errorf(
			"transition from %q to %q is not allowed: expected %q to retry the scrape",
			current, next, StatusScrapePending,
		)

	default:
		return fmt.Errorf(
			"unknown current status %q: cannot transition to %q",
			current, next,
		)
	}
}
