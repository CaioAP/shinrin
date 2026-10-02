package domain

import "errors"

// Sentinel errors returned across the ports. Adapters wrap or translate them
// (for example the HTTP adapter maps ErrNotFound to 404), callers test them
// with errors.Is.
var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid input")
)
