package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
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
		resolved, rerr := resolveGitHubLogin(c.Request.Context(), login)
		if rerr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "could not resolve login"})
			return
		}
		id, login = resolved.ID, resolved.Login
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

// No auth needed: public GitHub profiles are world-readable.
func resolveGitHubLogin(ctx context.Context, login string) (githubUser, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	u := "https://api.github.com/users/" + url.PathEscape(login)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return githubUser{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return githubUser{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return githubUser{}, errors.New("github users lookup non-200")
	}
	var gu githubUser
	if err := json.NewDecoder(resp.Body).Decode(&gu); err != nil || gu.ID == 0 {
		return githubUser{}, errors.New("github users decode")
	}
	return gu, nil
}
