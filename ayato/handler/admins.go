package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

func (h *Handler) AdminsListHandler(c *gin.Context) {
	admins, err := h.s.ListAdmins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list"})
		return
	}
	if admins == nil {
		admins = []domain.AllowedAdmin{}
	}
	c.JSON(http.StatusOK, gin.H{"admins": admins})
}

// Accepts a numeric id, or a GitHub login resolved to an id via the GitHub API.
func (h *Handler) AdminsAddHandler(c *gin.Context) {
	var body struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	id, login := body.ID, body.Login
	if id <= 0 {
		if login == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id or login required"})
			return
		}
		resolvedID, resolvedLogin, rerr := h.s.ResolveGitHubLogin(c.Request.Context(), login)
		if rerr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "could not resolve login"})
			return
		}
		id, login = resolvedID, resolvedLogin
	}
	if err := h.s.AddAdmin(id, login); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "add"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "login": login})
}

func (h *Handler) AdminsRemoveHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	// Refuse to empty the allowlist (including self-removal): auth fails closed on
	// an empty list and the bootstrap admin is only re-seeded at startup, so this
	// would lock everyone out until a restart.
	admins, err := h.s.ListAdmins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list"})
		return
	}
	if len(admins) == 1 && admins[0].ID == id {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot remove the last admin"})
		return
	}
	if err := h.s.RemoveAdmin(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "remove"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
