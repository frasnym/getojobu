package constants

import "fmt"

// JobStatus is the typed status of a job listing.
type JobStatus string

// Job status constants.
const (
	StatusNew            JobStatus = "new"
	StatusAnalyzePending JobStatus = "analyze_pending"
	StatusAnalyzed       JobStatus = "analyzed"
	StatusFailed         JobStatus = "failed"
)

// ValidateJobStatusTransition returns an error when transitioning from current
// to next is not permitted. Allowed paths:
//
//	""              → analyze_pending  (blank / unset treated as new)
//	new             → analyze_pending
//	analyze_pending → analyzed
//	analyze_pending → failed
//	failed          → analyze_pending  (retry)
//
// Everything else is rejected, including any transition out of "analyzed".
func ValidateJobStatusTransition(current, next JobStatus) error {
	// Normalize a blank current status to "new" so callers don't have to.
	if current == "" {
		current = StatusNew
	}

	switch current {
	case StatusNew:
		if next == StatusAnalyzePending {
			return nil
		}
		return fmt.Errorf(
			"transition from %q to %q is not allowed: job must move to %q first",
			current, next, StatusAnalyzePending,
		)

	case StatusAnalyzePending:
		if next == StatusAnalyzed || next == StatusFailed {
			return nil
		}
		return fmt.Errorf(
			"transition from %q to %q is not allowed: expected %q or %q",
			current, next, StatusAnalyzed, StatusFailed,
		)

	case StatusAnalyzed:
		return fmt.Errorf(
			"transition from %q to %q is not allowed: job is already finalized",
			current, next,
		)

	case StatusFailed:
		if next == StatusAnalyzePending {
			return nil
		}
		return fmt.Errorf(
			"transition from %q to %q is not allowed: failed jobs may only be retried via %q",
			current, next, StatusAnalyzePending,
		)

	default:
		return fmt.Errorf(
			"unknown current status %q: cannot transition to %q",
			current, next,
		)
	}
}
