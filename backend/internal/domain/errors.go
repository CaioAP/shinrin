package domain

import "errors"

// Sentinel errors returned across the ports. Adapters wrap or translate them
// (for example the HTTP adapter maps ErrNotFound to 404), callers test them
// with errors.Is.
var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid input")
	// ErrConflict means the input clashes with stored state (an email that
	// already has an account, a watchlist name already in use).
	ErrConflict = errors.New("conflict")
	// ErrUnauthorized means the caller is not signed in, or the credentials
	// or session are wrong or expired. It never says which.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrRateLimited means too many attempts in a short time.
	ErrRateLimited = errors.New("too many attempts, try again later")
)
