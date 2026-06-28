package aur

import (
	"log/slog"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
)

// Handler exposes the admin-only management surface for registered AUR sources.
type Handler struct {
	b *Backend
}

// NewHandler builds the management handler over a Backend.
func NewHandler(b *Backend) *Handler { return &Handler{b: b} }

type registerRequest struct {
	GitURL     string `json:"git_url"`
	Ref        string `json:"ref"`
	Maintainer string `json:"maintainer"`
}

// RegisterHandler registers (or re-registers) an external PKGBUILD git repo by
// parsing its .SRCINFO. POST body: {git_url, ref?, maintainer?}.
func (h *Handler) RegisterHandler(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.GitURL == "" {
		c.JSON(http.StatusBadRequest, domain.APIError{Message: "git_url is required"})
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

// CatalogHandler returns the sara-facing catalog (managed packages + sources).
// It is public: sara reads it without admin credentials.
func (h *Handler) CatalogHandler(c *gin.Context) {
	cat, err := h.b.Catalog(c.Request.Context())
	if err != nil {
		// Public endpoint: log the cause, don't leak it in the response body.
		slog.Error("AUR catalog failed", "error", err)
		c.JSON(http.StatusInternalServerError, domain.APIError{Message: "failed to build catalog"})
		return
	}
	c.JSON(http.StatusOK, cat)
}

// ListHandler returns the registered pkgbases.
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
