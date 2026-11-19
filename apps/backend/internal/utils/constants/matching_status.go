package constants

import "fmt"

// MatchingEvaluationStatus represents the lifecycle state of a compatibility evaluation.
type MatchingEvaluationStatus string

const (
	MatchingStatusNew         MatchingEvaluationStatus = "new"
	MatchingStatusQueued      MatchingEvaluationStatus = "queued"
	MatchingStatusQueuedRetry MatchingEvaluationStatus = "queued_retry"
	MatchingStatusPending     MatchingEvaluationStatus = "pending"
	MatchingStatusSuccess     MatchingEvaluationStatus = "success"
	MatchingStatusFailed      MatchingEvaluationStatus = "failed"
)

// ValidateMatchingStatusTransition checks whether moving from current to next is permitted.
// "new" is insert-only; "queued" follows "new", "queued_retry" follows "failed", and
// "pending" follows "new", "queued", or "queued_retry".
func ValidateMatchingStatusTransition(current, next MatchingEvaluationStatus) error {
	switch next {
	case MatchingStatusNew:
		return fmt.Errorf(
			"transition from %q to %q is not allowed: %q is an initial status",
			current, next, next,
		)
	case MatchingStatusQueued:
		if current != MatchingStatusNew {
			return fmt.Errorf(
				"transition from %q to %q is not allowed: evaluation must be %q",
				current, next, MatchingStatusNew,
			)
		}
	case MatchingStatusQueuedRetry:
		if current != MatchingStatusFailed {
			return fmt.Errorf(
				"transition from %q to %q is not allowed: evaluation must be %q",
				current, next, MatchingStatusFailed,
			)
		}
	case MatchingStatusPending:
		if current != MatchingStatusNew && current != MatchingStatusQueued && current != MatchingStatusQueuedRetry {
			return fmt.Errorf(
				"transition from %q to %q is not allowed: evaluation must be %q, %q, or %q",
				current, next, MatchingStatusNew, MatchingStatusQueued, MatchingStatusQueuedRetry,
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

// CoverLetterStatus represents the lifecycle state of cover letter generation.
type CoverLetterStatus string

const (
	CoverLetterStatusQueued      CoverLetterStatus = "queued"
	CoverLetterStatusQueuedRetry CoverLetterStatus = "queued_retry"
	CoverLetterStatusPending     CoverLetterStatus = "pending"
	CoverLetterStatusSuccess     CoverLetterStatus = "success"
	CoverLetterStatusFailed      CoverLetterStatus = "failed"
)

// IsValidCoverLetterStatus reports whether v is a known cover-letter status.
func IsValidCoverLetterStatus(v string) bool {
	switch CoverLetterStatus(v) {
	case CoverLetterStatusQueued, CoverLetterStatusQueuedRetry, CoverLetterStatusPending,
		CoverLetterStatusSuccess, CoverLetterStatusFailed:
		return true
	default:
		return false
	}
}

// CoverLetterVariant selects the prompt used for cover letter generation.
type CoverLetterVariant string

const (
	CoverLetterVariantShort CoverLetterVariant = "short"
	CoverLetterVariantFull  CoverLetterVariant = "full"
)

// IsValidCoverLetterVariant reports whether v is a supported cover-letter variant.
func IsValidCoverLetterVariant(v string) bool {
	switch CoverLetterVariant(v) {
	case CoverLetterVariantShort, CoverLetterVariantFull:
		return true
	default:
		return false
	}
}
