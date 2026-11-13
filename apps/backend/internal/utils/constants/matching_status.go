package constants

// MatchingEvaluationStatus represents the lifecycle state of a compatibility evaluation.
type MatchingEvaluationStatus string

const (
	MatchingStatusNew     MatchingEvaluationStatus = "new"
	MatchingStatusPending MatchingEvaluationStatus = "pending"
	MatchingStatusSuccess MatchingEvaluationStatus = "success"
	MatchingStatusFailed  MatchingEvaluationStatus = "failed"
)

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
