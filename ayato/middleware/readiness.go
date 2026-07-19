package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/platform"
)

type readiness interface {
	Ready() bool
}

// RejectMutationsWhenNotReady lets probes and reads continue during draining,
// while preventing a new write from starting after graceful shutdown begins.
func (m *Middleware) RejectMutationsWhenNotReady(state readiness) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		switch ctx.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			ctx.Next()
			return
		}
		if state == nil || state.Ready() {
			ctx.Next()
			return
		}
		ctx.AbortWithStatusJSON(
			http.StatusServiceUnavailable,
			platform.NewHTTPError(http.StatusServiceUnavailable, "service is draining"),
		)
	}
}
