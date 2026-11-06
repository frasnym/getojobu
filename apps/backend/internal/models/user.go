package models

import "database/sql"

// Role represents a user's role in the system.
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// User is the internal application user record mirrored from Clerk.
type User struct {
	ID                    string
	ClerkUserID           string
	Email                 sql.NullString
	FullName              sql.NullString
	Role                  Role
	OnboardingCompletedAt sql.NullTime
	DeletedAt             sql.NullTime
}
