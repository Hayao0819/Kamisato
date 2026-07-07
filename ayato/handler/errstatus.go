package handler

import (
	"errors"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

// errToStatus maps a domain error to its HTTP status. The single place transport
// decides a status, so handlers do not each invent their own mapping; anything
// unclassified is a 500.
func errToStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, domain.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, domain.ErrNotImplemented):
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
