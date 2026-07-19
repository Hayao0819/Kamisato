package signer

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/internal/limits"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// Handler builds the signer service: a single detach-sign endpoint guarded by the
// API-key verifier. The private key lives here (via signer), so the build workers
// that call it stay keyless.
func Handler(signer sign.Signer, verifier *apikey.Verifier, maxSize int) *gin.Engine {
	e := gin.New()
	e.Use(gin.Recovery())
	e.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	api := e.Group("/api/unstable")
	api.Use(verifier.Middleware(apikey.ScopeSign))
	api.POST("/sign", signHandler(signer, limits.PackageBytes(maxSize)))
	return e
}

// signHandler stages the POSTed body in a temp file (Signer works on a path) and
// returns the detached signature.
func signHandler(signer sign.Signer, maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		dir, err := os.MkdirTemp("", "miko-sign-*")
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		defer func() { _ = os.RemoveAll(dir) }()

		pkgPath := filepath.Join(dir, "artifact")
		f, err := os.Create(pkgPath)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		body := http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		if _, err := io.Copy(f, body); err != nil {
			_ = f.Close()
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				c.Status(http.StatusRequestEntityTooLarge)
				return
			}
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		if err := f.Close(); err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		sigPath, err := signer.Sign(c.Request.Context(), pkgPath)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		sig, err := os.ReadFile(sigPath)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		c.Data(http.StatusOK, "application/pgp-signature", sig)
	}
}
