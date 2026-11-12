package models

import "time"

// UserPreference represents a user's search preferences for job scraping.
type UserPreference struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Keywords  []string  `json:"keywords"`
	Locations []string  `json:"locations"`
	Boards    []string  `json:"boards"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
