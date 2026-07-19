// Package httpapi defines Ayato's transport-level HTTP representations.
//
// Domain and service packages deliberately do not depend on these types. They
// report semantic errors through domain sentinels; handlers choose the HTTP
// status and serialize this stable public envelope.
package httpapi

import "net/http"

type ErrorCode string

const (
	CodeRequestFailed      ErrorCode = "request_failed"
	CodeInvalidRequest     ErrorCode = "invalid_request"
	CodeUnauthorized       ErrorCode = "unauthorized"
	CodeForbidden          ErrorCode = "forbidden"
	CodeNotFound           ErrorCode = "not_found"
	CodeConflict           ErrorCode = "conflict"
	CodePayloadTooLarge    ErrorCode = "payload_too_large"
	CodeRateLimited        ErrorCode = "rate_limited"
	CodeInternal           ErrorCode = "internal_error"
	CodeNotImplemented     ErrorCode = "not_implemented"
	CodeUpstream           ErrorCode = "upstream_error"
	CodeServiceUnavailable ErrorCode = "service_unavailable"
)

// ErrorResponse is the public error envelope for non-protocol Ayato APIs.
// Message is safe for clients; implementation causes belong in server logs.
type ErrorResponse struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func NewError(status int, message string) ErrorResponse {
	return ErrorResponse{Code: CodeForStatus(status), Message: message}
}

func CodeForStatus(status int) ErrorCode {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return CodeInvalidRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusRequestEntityTooLarge:
		return CodePayloadTooLarge
	case http.StatusTooManyRequests:
		return CodeRateLimited
	case http.StatusNotImplemented:
		return CodeNotImplemented
	case http.StatusBadGateway:
		return CodeUpstream
	case http.StatusServiceUnavailable:
		return CodeServiceUnavailable
	default:
		if status >= http.StatusInternalServerError {
			return CodeInternal
		}
		return CodeRequestFailed
	}
}
