package constants

import "fmt"

// MatchingEvaluationStatus represents the lifecycle state of a compatibility evaluation.
type MatchingEvaluationStatus string

const (
	MatchingStatusQueued      MatchingEvaluationStatus = "queued"
	MatchingStatusQueuedRetry MatchingEvaluationStatus = "queued_retry"
	MatchingStatusPending     MatchingEvaluationStatus = "pending"
	MatchingStatusSuccess     MatchingEvaluationStatus = "success"
	MatchingStatusFailed      MatchingEvaluationStatus = "failed"
)

// ValidateMatchingStatusTransition checks whether moving from current to next is permitted.
// Moving to "pending" is only allowed if current status is "queued" or "queued_retry".
func ValidateMatchingStatusTransition(current, next MatchingEvaluationStatus) error {
	switch next {
	case MatchingStatusPending:
		if current != MatchingStatusQueued && current != MatchingStatusQueuedRetry {
			return fmt.Errorf(
				"transition from %q to %q is not allowed: evaluation must be %q or %q",
				current, next, MatchingStatusQueued, MatchingStatusQueuedRetry,
			)
		}
	}
	return nil
}

// EligibilityStatus represents the aggregate eligibility result from mandatory requirements.
type EligibilityStatus string

const (
	EligibilityStatusEligible          EligibilityStatus = "eligible"
	EligibilityStatusNotEligible       EligibilityStatus = "not_eligible"
	EligibilityStatusNeedsVerification EligibilityStatus = "needs_verification"
)

// RecommendationLabel represents the deterministic recommendation based on score and eligibility.
type RecommendationLabel string

const (
	RecommendationStrongMatch   RecommendationLabel = "strong_match"
	RecommendationWorthApplying RecommendationLabel = "worth_applying"
	RecommendationPossibleMatch RecommendationLabel = "possible_match"
	RecommendationLowMatch      RecommendationLabel = "low_match"
)

// RequirementStatus represents the assessment of a mandatory requirement.
type RequirementStatus string

const (
	RequirementStatusMet       RequirementStatus = "met"
	RequirementStatusNotMet    RequirementStatus = "not_met"
	RequirementStatusUncertain RequirementStatus = "uncertain"
)

// ReportStatus represents the lifecycle state of a match report.
type ReportStatus string

const (
	ReportStatusQueued      ReportStatus = "queued"
	ReportStatusQueuedRetry ReportStatus = "queued_retry"
	ReportStatusPending     ReportStatus = "pending"
	ReportStatusSuccess     ReportStatus = "success"
	ReportStatusFailed      ReportStatus = "failed"
)

// ResumeStatus represents the lifecycle state of resume generation.
type ResumeStatus string

const (
	ResumeStatusQueued      ResumeStatus = "queued"
	ResumeStatusQueuedRetry ResumeStatus = "queued_retry"
	ResumeStatusPending     ResumeStatus = "pending"
	ResumeStatusSuccess     ResumeStatus = "success"
	ResumeStatusFailed      ResumeStatus = "failed"
)
