package handler

import (
	"fmt"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
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
// stable flow. It is admin-gated: promotion publishes to a downstream tier, which
// is a deliberate release action kept separate from building/uploading.
func (h *Handler) PromoteHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "repository name is required"})
		return
	}
	var req promoteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("invalid request body: %s", err.Error())})
		return
	}
	if req.Pkgname == "" || req.From == "" || req.To == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "from, to and pkgname are required"})
		return
	}
	if err := h.s.PromotePackage(ctx.Request.Context(), repoName, conf.Tier(req.From), conf.Tier(req.To), req.Pkgname, req.Version); err != nil {
		ctx.JSON(errToStatus(err), domain.APIError{
			Message: "promote package err",
			Reason:  err.Error(),
		})
		return
	}
	ctx.String(http.StatusOK, fmt.Sprintf("'%s' promoted from %s to %s in %s", req.Pkgname, req.From, req.To, repoName))
}
