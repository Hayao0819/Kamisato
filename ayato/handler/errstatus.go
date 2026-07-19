package handler

import (
	"log/slog"
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
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

func respondError(ctx *gin.Context, status int, message string) {
	ctx.JSON(status, platform.NewHTTPError(status, message))
}

// respondServiceError converts errors deliberately classified by the domain
// into their client status. Unexpected failures are logged and get only the
// caller-supplied safe message.
func respondServiceError(ctx *gin.Context, operation, publicMessage string, err error) {
	status := errToStatus(err)
	if status >= http.StatusInternalServerError && status != http.StatusNotImplemented {
		respondLoggedError(ctx, status, operation, publicMessage, err)
	} else {
		respondError(ctx, status, publicMessage)
	}
}

func respondLoggedError(ctx *gin.Context, status int, operation, publicMessage string, err error) {
	slog.Error(operation, "error", err)
	respondError(ctx, status, publicMessage)
}
