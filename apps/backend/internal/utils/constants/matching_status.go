package constants

import "fmt"

// MatchingEvaluationStatus represents the lifecycle state of a compatibility evaluation.
type MatchingEvaluationStatus string

const (
	MatchingStatusNew      MatchingEvaluationStatus = "new"
	MatchingStatusNewRetry MatchingEvaluationStatus = "new-retry"
	MatchingStatusPending  MatchingEvaluationStatus = "pending"
	MatchingStatusSuccess  MatchingEvaluationStatus = "success"
	MatchingStatusFailed   MatchingEvaluationStatus = "failed"
)

// ValidateMatchingStatusTransition checks whether moving from current to next is permitted.
// Moving to "pending" is only allowed if current status is "new" or "new-retry".
func ValidateMatchingStatusTransition(current, next MatchingEvaluationStatus) error {
	switch next {
	case MatchingStatusPending:
		if current != MatchingStatusNew && current != MatchingStatusNewRetry {
			return fmt.Errorf(
				"transition from %q to %q is not allowed: evaluation must be %q or %q",
				current, next, MatchingStatusNew, MatchingStatusNewRetry,
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
	ReportStatusPending      ReportStatus = "pending"
	ReportStatusPendingRetry ReportStatus = "pending-retry"
	ReportStatusGenerated    ReportStatus = "generated"
	ReportStatusFailed       ReportStatus = "failed"
)
