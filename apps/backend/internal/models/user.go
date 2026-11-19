package models

import (
	"encoding/json"
	"time"
)

// Role represents a user's role in the system.
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// User is the internal application user record mirrored from Clerk.
type User struct {
	ID                    string          `json:"id"`
	ClerkUserID           string          `json:"clerk_user_id"`
	Email                 *string         `json:"email,omitempty"`
	FullName              *string         `json:"full_name,omitempty"`
	Role                  Role            `json:"role"`
	OnboardingCompletedAt *time.Time      `json:"onboarding_completed_at,omitempty"`
	DeletedAt             *time.Time      `json:"deleted_at,omitempty"`
	Headline              *string         `json:"headline,omitempty"`
	Phone                 *string         `json:"phone,omitempty"`
	PortfolioURL          *string         `json:"portfolio_url,omitempty"`
	LinkedInURL           *string         `json:"linkedin_url,omitempty"`
	GitHubURL             *string         `json:"github_url,omitempty"`
	ProfileData           json.RawMessage `json:"-"`
	FactsData             json.RawMessage `json:"-"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

// RefreshProfileResult is the per-user outcome of a bulk profile cache refresh.
type RefreshProfileResult struct {
	UserID string  `json:"user_id"`
	Error  *string `json:"error,omitempty"`
}

// UserProfileInput holds the profile fields a user may edit on their own
// account. Pointer fields allow a caller to clear a value with null.
type UserProfileInput struct {
	FullName     *string `json:"full_name"`
	Headline     *string `json:"headline"`
	Phone        *string `json:"phone"`
	PortfolioURL *string `json:"portfolio_url"`
	LinkedInURL  *string `json:"linkedin_url"`
	GitHubURL    *string `json:"github_url"`
}
