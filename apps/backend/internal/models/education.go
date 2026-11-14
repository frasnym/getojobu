package models

import (
	"time"

	"github.com/google/uuid"
)

// UserEducation represents an education entry for a user.
type UserEducation struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	Institution  string    `json:"institution"`
	Degree       string    `json:"degree"`
	FieldOfStudy string    `json:"field_of_study"`
	StartYear    *int      `json:"start_year,omitempty"`
	EndYear      *int      `json:"end_year,omitempty"`
	GPA          *string   `json:"gpa,omitempty"`
	IsCurrent    bool      `json:"is_current"`
	SortOrder    int       `json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
