package models

import "time"

// MeResponse is the account-details payload for the authenticated user.
// It intentionally omits clerk_user_id and all internal-only fields.
type MeResponse struct {
	ID                    string     `json:"id"`
	Email                 *string    `json:"email,omitempty"`
	FullName              *string    `json:"full_name,omitempty"`
	Headline              *string    `json:"headline,omitempty"`
	Phone                 *string    `json:"phone,omitempty"`
	PortfolioURL          *string    `json:"portfolio_url,omitempty"`
	LinkedInURL           *string    `json:"linkedin_url,omitempty"`
	GitHubURL             *string    `json:"github_url,omitempty"`
	Role                  Role       `json:"role"`
	OnboardingCompletedAt *time.Time `json:"onboarding_completed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}
