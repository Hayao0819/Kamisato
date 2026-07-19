package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

// promoteRequest promotes one package by name (and optionally a pinned version)
// between two tiers of a tiered repo.
type promoteRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Pkgname string `json:"pkgname"`
	Version string `json:"version,omitempty"`
}

// PromoteHandler advances a package through a tiered repo's staging -> testing ->
// stable flow. Admin-gated: promotion is a deliberate release action, kept separate
// from building/uploading.
func (h *Handler) PromoteHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		respondError(ctx, http.StatusBadRequest, "repository name is required")
		return
	}
	var req promoteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respondError(ctx, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Pkgname == "" || req.From == "" || req.To == "" {
		respondError(ctx, http.StatusBadRequest, "from, to and pkgname are required")
		return
	}
	from, err := domain.ParseTier(req.From)
	if err != nil {
		respondError(ctx, http.StatusBadRequest, err.Error())
		return
	}
	to, err := domain.ParseTier(req.To)
	if err != nil {
		respondError(ctx, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.s.PromotePackage(ctx.Request.Context(), repoName, from, to, req.Pkgname, req.Version); err != nil {
		respondServiceError(ctx, "promote package", "failed to promote package", err)
		return
	}
	ctx.String(http.StatusOK, fmt.Sprintf("'%s' promoted from %s to %s in %s", req.Pkgname, req.From, req.To, repoName))
}

// SyncUpstreamHandler refreshes an upstream-layered repo from its upstream database
// and rebuilds the served merged view. Admin-gated because it rewrites the served db.
func (h *Handler) SyncUpstreamHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		respondError(ctx, http.StatusBadRequest, "repository name is required")
		return
	}
	res, err := h.s.SyncUpstream(ctx.Request.Context(), repoName)
	if err != nil {
		respondServiceError(ctx, "sync upstream", "failed to sync upstream", err)
		return
	}
	ctx.JSON(http.StatusOK, res)
}
