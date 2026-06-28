package aur

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	"github.com/Hayao0819/Kamisato/internal/kayoproto"
	"github.com/gin-gonic/gin"
)

// Handler is the admin-only management surface for registered AUR sources and the
// kayo-facing catalog.
type Handler struct {
	b      *Backend
	signer *CatalogSigner
}

// A nil signer serves the catalog unsigned (legacy); kayo refuses that for any
// pinned source.
func NewHandler(b *Backend, signer *CatalogSigner) *Handler {
	return &Handler{b: b, signer: signer}
}

type registerRequest struct {
	GitURL     string `json:"git_url"`
	Ref        string `json:"ref"`
	Maintainer string `json:"maintainer"`
}

// RegisterHandler registers an external PKGBUILD repo. Body: {git_url, ref?, maintainer?}.
func (h *Handler) RegisterHandler(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.GitURL == "" {
		c.JSON(http.StatusBadRequest, domain.APIError{Message: "git_url is required"})
		return
	}

	// Validate here only to answer 400 (client error) instead of a blanket 502;
	// Register re-validates before cloning, so this is not the trust boundary.
	if err := gitcmd.ValidateRemote(req.GitURL); err != nil {
		c.JSON(http.StatusBadRequest, domain.APIError{Message: "invalid git_url", Reason: err.Error()})
		return
	}

	pkgbase, names, err := h.b.Register(c.Request.Context(), req.GitURL, req.Ref, req.Maintainer)
	if err != nil {
		slog.Error("AUR register failed", "git_url", req.GitURL, "error", err)
		c.JSON(http.StatusBadGateway, domain.APIError{Message: "failed to register source", Reason: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pkgbase": pkgbase, "packages": names})
}

// CatalogHandler serves the catalog as a signed envelope. It is public: kayo
// verifies the signature instead of credentials. Payload is a json.RawMessage so
// c.JSON re-emits the exact bytes ayato signed.
func (h *Handler) CatalogHandler(c *gin.Context) {
	cat, err := h.b.Catalog(c.Request.Context())
	if err != nil {
		// Public endpoint: log the cause, don't leak it in the response body.
		slog.Error("AUR catalog failed", "error", err)
		c.JSON(http.StatusInternalServerError, domain.APIError{Message: "failed to build catalog"})
		return
	}

	if h.signer == nil {
		payload, _ := json.Marshal(kayoproto.SignedPayload{IssuedAt: time.Now().UTC(), Catalog: cat})
		c.JSON(http.StatusOK, kayoproto.CatalogEnvelope{Payload: payload, Alg: "none"})
		return
	}
	env, err := h.signer.Sign(cat)
	if err != nil {
		slog.Error("AUR catalog sign failed", "error", err)
		c.JSON(http.StatusInternalServerError, domain.APIError{Message: "failed to sign catalog"})
		return
	}
	c.JSON(http.StatusOK, env)
}

// PubkeyHandler publishes the signing public key for TOFU bootstrap and tooling.
func (h *Handler) PubkeyHandler(c *gin.Context) {
	if h.signer == nil {
		c.JSON(http.StatusNotFound, domain.APIError{Message: "catalog signing is not enabled"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"alg": "ed25519", "key_id": h.signer.KeyID(), "pubkey": h.signer.PublicKeyB64()})
}

func (h *Handler) ListHandler(c *gin.Context) {
	bases, err := h.b.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.APIError{Message: "failed to list sources", Reason: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pkgbases": bases})
}

// RemoveHandler deregisters a pkgbase and drops its derived metadata.
func (h *Handler) RemoveHandler(c *gin.Context) {
	pkgbase := c.Param("pkgbase")
	if pkgbase == "" {
		c.JSON(http.StatusBadRequest, domain.APIError{Message: "pkgbase is required"})
		return
	}
	if err := h.b.Remove(c.Request.Context(), pkgbase); err != nil {
		c.JSON(http.StatusInternalServerError, domain.APIError{Message: "failed to remove source", Reason: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
