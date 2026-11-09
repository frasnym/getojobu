package models

import (
	"time"
)

// ExperienceType represents the type of a user experience.
type ExperienceType string

const (
	ExperienceTypeProfessional ExperienceType = "professional"
	ExperienceTypeFreelance    ExperienceType = "freelance"
	ExperienceTypeInternship   ExperienceType = "internship"
	ExperienceTypeVolunteer    ExperienceType = "volunteer"
	ExperienceTypeOther        ExperienceType = "other"
)

// IsValid reports whether the experience type is one of the allowed values.
func (e ExperienceType) IsValid() bool {
	switch e {
	case ExperienceTypeProfessional, ExperienceTypeFreelance, ExperienceTypeInternship, ExperienceTypeVolunteer, ExperienceTypeOther:
		return true
	default:
		return false
	}
}

// Experience is a user's employment or other career experience.
type Experience struct {
	ID             string
	UserID         string
	ExperienceType ExperienceType
	Organization   string
	RoleTitle      string
	Location       string
	Description    string
	StartedAt      time.Time
	EndedAt        time.Time
	IsCurrent      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CreateExperienceParams holds parameters for creating an experience.
type CreateExperienceParams struct {
	UserID         string
	ExperienceType ExperienceType
	Organization   string
	RoleTitle      string
	Location       string
	Description    string
	StartedAt      *string
	EndedAt        *string
	IsCurrent      bool
}

// GetExperienceParams holds parameters for getting an experience by ID.
type GetExperienceParams struct {
	UserID string
	ID     string
}

// UpdateExperienceParams holds parameters for updating an experience.
type UpdateExperienceParams struct {
	UserID         string
	ID             string
	ExperienceType ExperienceType
	Organization   string
	RoleTitle      string
	Location       string
	Description    string
	StartedAt      *string
	EndedAt        *string
	IsCurrent      bool
}
