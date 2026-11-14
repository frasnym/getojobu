package models

// Principal is the authenticated actor identity attached to the request context.
// It is resolved either from a Clerk session (local user) or from a machine API
// key (server-side key-to-identity mapping), and carried on the request context
// so downstream handlers and usecases can identify the caller without
// re-fetching the user record.
type Principal struct {
	ID                  string
	ClerkUserID         string
	Email               string
	FullName            string
	Role                Role
	OnboardingCompleted bool
	ActorType           string
	ActorID             string
}

// NewPrincipalFromUser builds a Principal from a User repository record.
func NewPrincipalFromUser(u *User) *Principal {
	email := ""
	if u.Email != nil {
		email = *u.Email
	}
	fullName := ""
	if u.FullName != nil {
		fullName = *u.FullName
	}
	return &Principal{
		ID:                  u.ID,
		ClerkUserID:         u.ClerkUserID,
		Email:               email,
		FullName:            fullName,
		Role:                u.Role,
		OnboardingCompleted: u.OnboardingCompletedAt != nil,
	}
}

// AuditActor returns the (actor_type, actor_id) pair for audit attribution.
// Clerk users return ("user", p.ID). Machine actors return (p.ActorType, p.ActorID).
func (p *Principal) AuditActor() (actorType, actorID string) {
	if p.ActorType != "" {
		return p.ActorType, p.ActorID
	}
	return "user", p.ID
}
