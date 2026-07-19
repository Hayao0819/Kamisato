package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

// Starts the web GitHub flow. No server-side state is written — the signed token
// IS the state.
func (h *AuthHandler) GitHubLoginHandler(c *gin.Context) {
	if !h.requireOAuth(c) {
		return
	}
	// Bind the flow to this browser: a random nonce rides a SameSite=Lax cookie and
	// only its hash is signed into the state, defeating login-CSRF / fixation
	// (RFC 6749 §10.12).
	nonce, err := auth.NewState()
	if err != nil {
		respondAuthError(c, http.StatusInternalServerError, "state")
		return
	}
	state, err := h.signer.Sign(auth.Claims{
		Typ:     auth.TypState,
		CLI:     false,
		Binding: auth.HashHex(nonce),
		Exp:     time.Now().Add(stateTTL),
	})
	if err != nil {
		respondAuthError(c, http.StatusInternalServerError, "state")
		return
	}
	scheme, _ := h.externalBase(c)
	h.setOAuthStateCookie(c, nonce, scheme == "https")
	c.Redirect(http.StatusFound, h.oauthConfig(c).AuthCodeURL(state))
}

// Starts the CLI flow. The loopback is reconstructed server-side from the integer
// port (never a full URL); ayaka's state rides inside the signed state token.
func (h *AuthHandler) CLIStartHandler(c *gin.Context) {
	if !h.requireOAuth(c) {
		return
	}
	portStr := c.Query("port")
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != portStr {
		respondAuthError(c, http.StatusBadRequest, "invalid port")
		return
	}
	challenge, cliState, ok := parseChallengeAndState(c)
	if !ok {
		return
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
		respondAuthError(c, http.StatusInternalServerError, "state")
		return
	}
	c.Redirect(http.StatusFound, h.oauthConfig(c).AuthCodeURL(state))
}

// parseChallengeAndState validates the PKCE challenge and resolves ayaka's
// optional state (generating one when absent), shared by the CLI and web start
// flows. On invalid input it writes the error response and returns ok=false.
func parseChallengeAndState(c *gin.Context) (challenge, cliState string, ok bool) {
	challenge = c.Query("challenge")
	if challenge == "" || len(challenge) > 256 {
		respondAuthError(c, http.StatusBadRequest, "missing or oversized challenge")
		return "", "", false
	}
	cliState = c.Query("state")
	if len(cliState) > 256 {
		respondAuthError(c, http.StatusBadRequest, "oversized state")
		return "", "", false
	}
	if cliState == "" {
		var err error
		if cliState, err = auth.NewState(); err != nil {
			respondAuthError(c, http.StatusInternalServerError, "state")
			return "", "", false
		}
	}
	return challenge, cliState, true
}

// Cross-origin web-bearer flow: PKCE (not a binding cookie) ties the code to the
// SPA, so no state cookie is set; the code returns via postMessage.
func (h *AuthHandler) WebStartHandler(c *gin.Context) {
	if !h.requireOAuth(c) {
		return
	}
	challenge, cliState, ok := parseChallengeAndState(c)
	if !ok {
		return
	}
	state, err := h.signer.Sign(auth.Claims{
		Typ:       auth.TypState,
		Web:       true,
		Challenge: challenge,
		CLIState:  cliState,
		Exp:       time.Now().Add(stateTTL),
	})
	if err != nil {
		respondAuthError(c, http.StatusInternalServerError, "state")
		return
	}
	c.Redirect(http.StatusFound, h.oauthConfig(c).AuthCodeURL(state))
}
