package domain

import "errors"

type APIError struct {
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func (e *APIError) Error() string {
	return e.Message
}

// ErrNotImplemented lets a handler answer 501 instead of a misleading success.
var ErrNotImplemented = errors.New("not implemented")

// ErrInvalidUpload marks an upload rejected for a client-side reason — a malformed
// package, a missing required signature, an unverifiable signature, or a bad arch
// — so a handler can answer 400 instead of a misleading 500.
var ErrInvalidUpload = errors.New("invalid upload")
