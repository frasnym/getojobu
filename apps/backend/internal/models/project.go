package models

import "time"

// Project is a user's project, initiative, or meaningful piece of work.
// It may be associated with an Experience, or standalone (UserExperienceID zero value).
type Project struct {
	ID               string
	UserID           string
	UserExperienceID string
	Name             string
	Description      string
	Responsibilities string
	Achievements     string
	StartedAt        time.Time
	EndedAt          time.Time
	IsCurrent        bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ProjectSkill links a project to a skill the user applied in that project.
type ProjectSkill struct {
	UserID        string
	UserProjectID string
	UserSkillID   string
	CreatedAt     time.Time
}

// UserSkill is a user's skill inventory entry referenced by ProjectSkill.
type UserSkill struct {
	ID          string
	UserID      string
	SkillName   string
	SkillKey    string
	Proficiency string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateProjectParams holds parameters for creating a project.
type CreateProjectParams struct {
	UserID           string
	Name             string
	Description      string
	Responsibilities string
	Achievements     string
	UserExperienceID *string
	StartedAt        *string
	EndedAt          *string
	IsCurrent        bool
}

// GetProjectParams holds parameters for getting a project by ID.
type GetProjectParams struct {
	UserID string
	ID     string
}

// UpdateProjectParams holds parameters for updating a project.
type UpdateProjectParams struct {
	UserID           string
	ID               string
	Name             string
	Description      string
	Responsibilities string
	Achievements     string
	UserExperienceID *string
	StartedAt        *string
	EndedAt          *string
	IsCurrent        bool
}

// LinkProjectSkillsParams holds parameters for linking skills to a project.
type LinkProjectSkillsParams struct {
	UserID    string
	ProjectID string
	SkillIDs  []string
}

// ListProjectSkillsParams holds parameters for listing skills of a project.
type ListProjectSkillsParams struct {
	UserID    string
	ProjectID string
}
