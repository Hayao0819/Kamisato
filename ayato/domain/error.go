package domain

import "errors"

type APIError struct {
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`
	// Code    int    `json:"code"`
}

func (e *APIError) Error() string {
	return e.Message
}

// ErrNotImplemented marks a feature that is not implemented yet, so a handler can
// answer 501 Not Implemented instead of a misleading success.
var ErrNotImplemented = errors.New("not implemented")
