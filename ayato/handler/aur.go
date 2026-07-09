package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/aur"
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
)

// AURHandler is the gin-facing surface for AUR source management and the
// kayo-facing catalog; parsing and responses live here so the ayato/aur backend
// stays free of any web framework.
type AURHandler struct {
	svc *aur.Service
}

func NewAURHandler(svc *aur.Service) *AURHandler {
	return &AURHandler{svc: svc}
}

type aurRegisterRequest struct {
	GitURL     string `json:"git_url"`
	Ref        string `json:"ref"`
	Maintainer string `json:"maintainer"`
}

// RegisterHandler registers an external PKGBUILD repo. Body: {git_url, ref?, maintainer?}.
func (h *AURHandler) RegisterHandler(c *gin.Context) {
	var req aurRegisterRequest
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

	pkgbase, names, err := h.svc.Register(c.Request.Context(), req.GitURL, req.Ref, req.Maintainer)
	if err != nil {
		slog.Error("AUR register failed", "git_url", req.GitURL, "error", err)
		c.JSON(http.StatusBadGateway, domain.APIError{Message: "failed to register source", Reason: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pkgbase": pkgbase, "packages": names})
}

// CatalogHandler serves the catalog as a signed envelope. Public: kayo verifies
// the signature instead of credentials.
func (h *AURHandler) CatalogHandler(c *gin.Context) {
	env, err := h.svc.Envelope(c.Request.Context())
	if err != nil {
		// Public endpoint: log the cause, don't leak it in the response body.
		slog.Error("AUR catalog failed", "error", err)
		c.JSON(http.StatusInternalServerError, domain.APIError{Message: "failed to build catalog"})
		return
	}
	c.JSON(http.StatusOK, env)
}

// PubkeyHandler publishes the signing public key for TOFU bootstrap and tooling.
func (h *AURHandler) PubkeyHandler(c *gin.Context) {
	if !h.svc.SignerEnabled() {
		c.JSON(http.StatusNotFound, domain.APIError{Message: "catalog signing is not enabled"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"alg": "ed25519", "key_id": h.svc.SignerKeyID(), "pubkey": h.svc.SignerPublicKeyB64()})
}

func (h *AURHandler) ListHandler(c *gin.Context) {
	bases, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.APIError{Message: "failed to list sources", Reason: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pkgbases": bases})
}

// RemoveHandler deregisters a pkgbase and drops its derived metadata.
func (h *AURHandler) RemoveHandler(c *gin.Context) {
	pkgbase := c.Param("pkgbase")
	if pkgbase == "" {
		c.JSON(http.StatusBadRequest, domain.APIError{Message: "pkgbase is required"})
		return
	}
	if err := h.svc.Remove(c.Request.Context(), pkgbase); err != nil {
		c.JSON(http.StatusInternalServerError, domain.APIError{Message: "failed to remove source", Reason: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
