package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/httpapi"
)

func TestErrToStatus(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, http.StatusOK},
		{"invalid", domain.ErrInvalid, http.StatusBadRequest},
		{"invalid-upload wraps invalid", domain.ErrInvalidUpload, http.StatusBadRequest},
		{"wrapped invalid", fmt.Errorf("bad arch: %w", domain.ErrInvalid), http.StatusBadRequest},
		{"not-found", domain.ErrNotFound, http.StatusNotFound},
		{"wrapped not-found", fmt.Errorf("%w: package foo", domain.ErrNotFound), http.StatusNotFound},
		{"conflict", domain.ErrConflict, http.StatusConflict},
		{"not-implemented", domain.ErrNotImplemented, http.StatusNotImplemented},
		{"unclassified", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		if got := errToStatus(tc.err); got != tc.want {
			t.Errorf("%s: errToStatus = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestRespondServiceErrorUsesSafePublicEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		err     error
		status  int
		code    httpapi.ErrorCode
		message string
	}{
		{
			name:    "classified not found",
			err:     fmt.Errorf("%w: storage key secret-bucket/key", domain.ErrNotFound),
			status:  http.StatusNotFound,
			code:    httpapi.CodeNotFound,
			message: "package not found",
		},
		{
			name:    "unclassified internal failure",
			err:     errors.New("database password secret-value"),
			status:  http.StatusInternalServerError,
			code:    httpapi.CodeInternal,
			message: "failed to load package",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			respondServiceError(ctx, "load package", tc.message, tc.err)

			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.status)
			}
			var response httpapi.ErrorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Code != tc.code || response.Message != tc.message {
				t.Errorf("response = %+v, want code=%q message=%q", response, tc.code, tc.message)
			}
			if strings.Contains(recorder.Body.String(), "secret") {
				t.Errorf("response leaked internal cause: %s", recorder.Body.String())
			}
		})
	}
}
