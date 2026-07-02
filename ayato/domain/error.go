package domain

import (
	"errors"
	"fmt"
)

type APIError struct {
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func (e *APIError) Error() string {
	return e.Message
}

// The error taxonomy below lets any layer signal the HTTP class a failure should
// map to, without importing net/http: a service/repository wraps its error with
// the matching sentinel and the transport layer's errToStatus resolves the code.
var (
	// ErrNotImplemented answers 501 instead of a misleading success.
	ErrNotImplemented = errors.New("not implemented")
	// ErrNotFound answers 404: a repo, package, or file that does not exist.
	ErrNotFound = errors.New("not found")
	// ErrInvalid answers 400: a client-side request the server refuses to act on.
	ErrInvalid = errors.New("invalid request")
	// ErrConflict answers 409: the request conflicts with the current state.
	ErrConflict = errors.New("conflict")
)

// ErrInvalidUpload marks an upload rejected for a client-side reason — a malformed
// package, a missing required signature, an unverifiable signature, or a bad arch
// — so a handler can answer 400. It is a specialization of ErrInvalid.
var ErrInvalidUpload = fmt.Errorf("%w: invalid upload", ErrInvalid)
