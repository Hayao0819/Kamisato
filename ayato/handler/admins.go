package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

func (h *AdminHandler) AdminsListHandler(c *gin.Context) {
	admins, err := h.admins.ListAdmins()
	if err != nil {
		respondAuthError(c, http.StatusInternalServerError, "list")
		return
	}
	if admins == nil {
		admins = []domain.AllowedAdmin{}
	}
	c.JSON(http.StatusOK, gin.H{"admins": admins})
}

// Accepts a numeric id, or a GitHub login resolved to an id via the GitHub API.
func (h *AdminHandler) AdminsAddHandler(c *gin.Context) {
	var body struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondAuthError(c, http.StatusBadRequest, "invalid request")
		return
	}
	id, login := body.ID, body.Login
	if id <= 0 {
		if login == "" {
			respondAuthError(c, http.StatusBadRequest, "id or login required")
			return
		}
		resolvedID, resolvedLogin, rerr := h.admins.ResolveGitHubLogin(c.Request.Context(), login)
		if rerr != nil {
			respondAuthError(c, http.StatusBadGateway, "could not resolve login")
			return
		}
		id, login = resolvedID, resolvedLogin
	}
	if err := h.admins.AddAdmin(id, login); err != nil {
		respondAuthError(c, http.StatusInternalServerError, "add")
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "login": login})
}

func (h *AdminHandler) AdminsRemoveHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondAuthError(c, http.StatusBadRequest, "invalid id")
		return
	}
	// Refuse to empty the allowlist (including self-removal): auth fails closed on
	// an empty list and the bootstrap admin is only re-seeded at startup, so this
	// would lock everyone out until a restart.
	admins, err := h.admins.ListAdmins()
	if err != nil {
		respondAuthError(c, http.StatusInternalServerError, "list")
		return
	}
	if len(admins) == 1 && admins[0].ID == id {
		respondAuthError(c, http.StatusConflict, "cannot remove the last admin")
		return
	}
	if err := h.admins.RemoveAdmin(id); err != nil {
		respondAuthError(c, http.StatusInternalServerError, "remove")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
