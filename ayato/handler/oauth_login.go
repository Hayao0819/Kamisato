package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/gin-gonic/gin"
)

// GitHubLoginHandler starts a WEB "Sign in with GitHub" flow: it mints a signed
// state token (carrying the browser binding) and redirects to GitHub's consent
// page. No server-side state is written — the signed token IS the state.
func (h *Handler) GitHubLoginHandler(c *gin.Context) {
	if !h.oauthEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "github login not configured"})
		return
	}
	// Bind the flow to THIS browser: a random nonce travels in a host-only
	// SameSite=Lax cookie and only its hash is signed into the state. The callback
	// requires the cookie to match, defeating login-CSRF / session fixation
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

// CLIStartHandler starts a CLI flow. ayaka opens this URL in the user's browser
// with the loopback port, a PKCE S256 challenge, and a state. The loopback URL
// is reconstructed server-side from the integer port (never a full URL); ayaka's
// original state is carried inside the signed state token so the callback can
// echo it back unchanged. The signed token IS the state sent to GitHub.
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

// WebStartHandler starts a cross-origin web-bearer flow for a static SPA. The
// SPA opens this in a popup with a PKCE S256 challenge and its own state nonce.
// Like the CLI flow, PKCE (not a browser-binding cookie) ties the eventual code
// to the SPA that began it, so no state cookie is set; the callback returns the
// one-time code to the SPA via postMessage rather than a redirect.
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
