package handler

import (
	"errors"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

// errToStatus maps a service/repository error to its HTTP status via the domain
// error taxonomy. The service translates infra sentinels (e.g. blob.ErrNotFound)
// into the domain taxonomy at its boundary, so transport only sees domain errors.
// Anything the lower layers did not classify is a 500. This is the single place
// transport decides a status, so handlers do not each invent their own (mis)mapping.
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
