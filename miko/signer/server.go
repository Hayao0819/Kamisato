package signer

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/internal/apikey"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// maxSignBytes bounds a signing request body so a buggy or hostile caller cannot
// exhaust disk staging the artifact. Generous enough for any real package.
const maxSignBytes = 2 << 30 // 2 GiB

// Handler builds the signer service: a single detach-sign endpoint guarded by the
// API-key verifier. The private key lives here (via signer), so the build workers
// that call it stay keyless.
func Handler(signer sign.Signer, verifier *apikey.Verifier) *gin.Engine {
	e := gin.New()
	e.Use(gin.Recovery())
	e.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	api := e.Group("/api/unstable")
	api.Use(verifier.Middleware())
	api.POST("sign", signHandler(signer))
	return e
}

// signHandler stages the POSTed body in a temp file (Signer works on a path) and
// returns the detached signature.
func signHandler(signer sign.Signer) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		if _, err := io.Copy(f, io.LimitReader(c.Request.Body, maxSignBytes)); err != nil {
			_ = f.Close()
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
