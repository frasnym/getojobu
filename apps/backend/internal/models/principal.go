package models

// Principal is the authenticated user identity attached to the request context.
// It is resolved from a Clerk session by the auth middleware and carried on
// the request context so downstream handlers and usecases can identify the
// caller without re-fetching the user record.
type Principal struct {
	ID                  string
	ClerkUserID         string
	Email               string
	FullName            string
	Role                Role
	OnboardingCompleted bool
}

// NewPrincipalFromUser builds a Principal from a User repository record.
func NewPrincipalFromUser(u *User) *Principal {
	return &Principal{
		ID:                  u.ID,
		ClerkUserID:         u.ClerkUserID,
		Email:               u.Email.String,
		FullName:            u.FullName.String,
		Role:                u.Role,
		OnboardingCompleted: u.OnboardingCompletedAt.Valid,
	}
}
