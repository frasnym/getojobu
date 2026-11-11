package constants

import "errors"

// ErrAccountDeactivated is returned when the local account is deactivated.
var ErrAccountDeactivated = errors.New("account deactivated")

// ErrUserNotFound is returned when no local user record exists for the
// given Clerk user ID. Users must be created via Clerk webhook before they
// can authenticate.
var ErrUserNotFound = errors.New("user not found")

// ErrNotFound is returned when no record exists for the given id and user.
var ErrNotFound = errors.New("not found")

// ErrInvalidTransition is returned when a requested job status transition is
// not permitted by the transition validator. The HTTP layer maps this to 422.
var ErrInvalidTransition = errors.New("invalid status transition")

// ErrNoActiveConnections is returned when no transaction is stored in context.
var ErrNoActiveConnections = errors.New("no active connections")
