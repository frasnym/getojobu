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
