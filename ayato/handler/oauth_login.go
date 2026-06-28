package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/gin-gonic/gin"
)

// Starts the web GitHub flow. No server-side state is written — the signed token
// IS the state.
func (h *Handler) GitHubLoginHandler(c *gin.Context) {
	if !h.oauthEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "github login not configured"})
		return
	}
	// Bind the flow to this browser: a random nonce rides a SameSite=Lax cookie and
	// only its hash is signed into the state, defeating login-CSRF / fixation
	// (RFC 6749 §10.12).
	nonce, err := auth.NewState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
		return
	}
	state, err := h.signer.Sign(auth.Claims{
		Typ:     auth.TypState,
		CLI:     false,
		Binding: auth.HashHex(nonce),
		Exp:     time.Now().Add(stateTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
		return
	}
	scheme, _ := h.externalBase(c)
	h.setOAuthStateCookie(c, nonce, scheme == "https")
	c.Redirect(http.StatusFound, h.oauthConfig(c).AuthCodeURL(state))
}

// Starts the CLI flow. The loopback is reconstructed server-side from the integer
// port (never a full URL); ayaka's state rides inside the signed state token.
func (h *Handler) CLIStartHandler(c *gin.Context) {
	if !h.oauthEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "github login not configured"})
		return
	}
	portStr := c.Query("port")
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != portStr {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid port"})
		return
	}
	challenge := c.Query("challenge")
	if challenge == "" || len(challenge) > 256 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or oversized challenge"})
		return
	}
	cliState := c.Query("state")
	if len(cliState) > 256 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "oversized state"})
		return
	}
	if cliState == "" {
		var serr error
		if cliState, serr = auth.NewState(); serr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
			return
		}
	}
	state, err := h.signer.Sign(auth.Claims{
		Typ:       auth.TypState,
		CLI:       true,
		Port:      port,
		Challenge: challenge,
		CLIState:  cliState,
		Exp:       time.Now().Add(stateTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
		return
	}
	c.Redirect(http.StatusFound, h.oauthConfig(c).AuthCodeURL(state))
}

// Cross-origin web-bearer flow: PKCE (not a binding cookie) ties the code to the
// SPA, so no state cookie is set; the code returns via postMessage.
func (h *Handler) WebStartHandler(c *gin.Context) {
	if !h.oauthEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "github login not configured"})
		return
	}
	challenge := c.Query("challenge")
	if challenge == "" || len(challenge) > 256 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or oversized challenge"})
		return
	}
	cliState := c.Query("state")
	if len(cliState) > 256 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "oversized state"})
		return
	}
	if cliState == "" {
		var serr error
		if cliState, serr = auth.NewState(); serr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
			return
		}
	}
	state, err := h.signer.Sign(auth.Claims{
		Typ:       auth.TypState,
		Web:       true,
		Challenge: challenge,
		CLIState:  cliState,
		Exp:       time.Now().Add(stateTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
		return
	}
	c.Redirect(http.StatusFound, h.oauthConfig(c).AuthCodeURL(state))
}
