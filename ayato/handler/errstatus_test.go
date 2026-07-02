package handler

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
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
		{"blob not-found", blob.ErrNotFound, http.StatusNotFound},
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
