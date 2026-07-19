package domain

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

// These sentinels let any layer signal an HTTP class without importing net/http;
// the transport layer's errToStatus resolves the code.
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

// ErrInvalidUpload specializes ErrInvalid for a rejected upload (bad package,
// missing/unverifiable signature, or bad arch).
var ErrInvalidUpload = fmt.Errorf("%w: invalid upload", ErrInvalid)
