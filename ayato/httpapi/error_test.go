package httpapi

import (
	"net/http"
	"testing"
)

func TestCodeForStatus(t *testing.T) {
	t.Parallel()
	tests := map[int]ErrorCode{
		http.StatusBadRequest:            CodeInvalidRequest,
		http.StatusUnauthorized:          CodeUnauthorized,
		http.StatusForbidden:             CodeForbidden,
		http.StatusNotFound:              CodeNotFound,
		http.StatusConflict:              CodeConflict,
		http.StatusRequestEntityTooLarge: CodePayloadTooLarge,
		http.StatusTooManyRequests:       CodeRateLimited,
		http.StatusInternalServerError:   CodeInternal,
		http.StatusNotImplemented:        CodeNotImplemented,
		http.StatusBadGateway:            CodeUpstream,
		http.StatusServiceUnavailable:    CodeServiceUnavailable,
		http.StatusTeapot:                CodeRequestFailed,
	}
	for status, want := range tests {
		status, want := status, want
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			if got := CodeForStatus(status); got != want {
				t.Fatalf("CodeForStatus(%d) = %q, want %q", status, got, want)
			}
		})
	}
}

func TestNewError(t *testing.T) {
	t.Parallel()
	got := NewError(http.StatusNotFound, "package not found")
	if got.Code != CodeNotFound || got.Message != "package not found" {
		t.Fatalf("NewError() = %+v", got)
	}
}
