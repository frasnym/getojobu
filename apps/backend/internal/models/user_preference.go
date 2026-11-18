package models

import "time"

// UserPreference represents a user's search preferences for job scraping.
type UserPreference struct {
	ID                         int64     `json:"id"`
	UserID                     string    `json:"user_id"`
	Keywords                   []string  `json:"keywords"`
	Locations                  []string  `json:"locations"`
	Boards                     []string  `json:"boards"`
	AutoHideLowMatchEvaluation bool      `json:"auto_hide_low_match_evaluation"`
	IsActive                   bool      `json:"is_active"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}
