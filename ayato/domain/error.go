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
