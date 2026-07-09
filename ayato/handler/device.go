package handler

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

const deviceVerifyPath = "/api/unstable/auth/device"

// deviceFormTmpl is the code-entry page; the code value is auto-escaped by
// html/template.
var deviceFormTmpl = template.Must(template.New("device").Parse(
	`<!doctype html><meta charset="utf-8"><title>Device login</title>
<body style="font-family:sans-serif;max-width:32rem;margin:4rem auto">
<h1>デバイス認証</h1>
<p>CLI に表示されたコードを入力してください。</p>
<form method="get" action="/api/unstable/auth/device/approve">
<input name="user_code" value="{{.UserCode}}" placeholder="XXXX-XXXX"
 autocapitalize="characters" autocomplete="off" style="font-size:1.4rem;padding:.4rem">
<button type="submit" style="font-size:1.4rem;padding:.4rem 1rem">続行</button>
</form>
{{if .Error}}<p style="color:#b00">{{.Error}}</p>{{end}}
</body>`))

// deviceMessageTmpl renders a terminal-state page (approved / denied / expired).
var deviceMessageTmpl = template.Must(template.New("devicemsg").Parse(
	`<!doctype html><meta charset="utf-8"><title>Device login</title>
<body style="font-family:sans-serif;max-width:32rem;margin:4rem auto">
<h1>{{.Title}}</h1><p>{{.Body}}</p></body>`))

func (h *Handler) renderDeviceForm(c *gin.Context, status int, userCode, errMsg string) {
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = deviceFormTmpl.Execute(c.Writer, struct{ UserCode, Error string }{userCode, errMsg})
}

func (h *Handler) renderDeviceMessage(c *gin.Context, status int, title, body string) {
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = deviceMessageTmpl.Execute(c.Writer, struct{ Title, Body string }{title, body})
}

// normalizeUserCode reformats input to canonical XXXX-XXXX so a code typed
// lowercase or without the dash still matches the stored value.
func normalizeUserCode(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	letters := b.String()
	if len(letters) == 8 {
		return letters[:4] + "-" + letters[4:]
	}
	return letters
}

// DeviceCodeHandler issues a device_code + user_code pair (RFC 8628 §3.2) so a
// browserless box can log in by having the user approve the code from any browser.
func (h *Handler) DeviceCodeHandler(c *gin.Context) {
	if !h.oauthEnabled() || h.device == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device login not configured"})
		return
	}
	deviceCode, err := auth.NewDeviceCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "device code"})
		return
	}
	userCode, err := auth.NewUserCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user code"})
		return
	}
	if err := h.device.CreateDevice(deviceCode, userCode, deviceCodeTTL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "device store"})
		return
	}
	_, base := h.externalBase(c)
	verifyURL := base + deviceVerifyPath
	c.JSON(http.StatusOK, gin.H{
		"device_code":               deviceCode,
		"user_code":                 userCode,
		"verification_uri":          verifyURL,
		"verification_uri_complete": verifyURL + "?user_code=" + url.QueryEscape(userCode),
		"expires_in":                int(deviceCodeTTL / time.Second),
		"interval":                  int(deviceInterval / time.Second),
	})
}

// DeviceVerifyHandler serves the code-entry page (verification_uri); a user_code
// query pre-fills it.
func (h *Handler) DeviceVerifyHandler(c *gin.Context) {
	if !h.oauthEnabled() || h.device == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device login not configured"})
		return
	}
	h.renderDeviceForm(c, http.StatusOK, normalizeUserCode(c.Query("user_code")), "")
}

// DeviceApproveHandler hands the entered user_code to the GitHub OAuth flow via
// the signed state; a binding cookie ties the flow to this browser (login-CSRF
// defense).
func (h *Handler) DeviceApproveHandler(c *gin.Context) {
	if !h.oauthEnabled() || h.device == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device login not configured"})
		return
	}
	userCode := normalizeUserCode(c.Query("user_code"))
	status, ok, err := h.device.LookupByUserCode(userCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "device store"})
		return
	}
	if !ok {
		h.renderDeviceForm(c, http.StatusNotFound, userCode, "コードが無効か期限切れです。CLI でやり直してください。")
		return
	}
	if status != auth.DevicePending {
		h.renderDeviceMessage(c, http.StatusOK, "処理済み", "このコードはすでに処理されています。")
		return
	}
	nonce, err := auth.NewState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
		return
	}
	state, err := h.signer.Sign(auth.Claims{
		Typ:      auth.TypState,
		Device:   true,
		UserCode: userCode,
		Binding:  auth.HashHex(nonce),
		Exp:      time.Now().Add(stateTTL),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state"})
		return
	}
	scheme, _ := h.externalBase(c)
	h.setOAuthStateCookie(c, nonce, scheme == "https")
	c.Redirect(http.StatusFound, h.oauthConfig(c).AuthCodeURL(state))
}

// finishDeviceLogin marks the pending authorization approved; called from the
// OAuth callback after the allowlist check passes.
func (h *Handler) finishDeviceLogin(c *gin.Context, st *auth.Claims, user githubUser) {
	if h.device == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device login not configured"})
		return
	}
	ok, err := h.device.ApproveDevice(st.UserCode, user.ID, user.Login)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "device store"})
		return
	}
	if !ok {
		h.renderDeviceMessage(c, http.StatusGone, "期限切れ", "このコードは期限切れです。CLI でやり直してください。")
		return
	}
	h.renderDeviceMessage(c, http.StatusOK, "承認しました", "ターミナルに戻ってください。このタブは閉じて構いません。")
}

// denyDeviceLogin records the rejection (user not allowlisted) so the polling
// client stops with access_denied rather than pending until the code expires.
func (h *Handler) denyDeviceLogin(c *gin.Context, st *auth.Claims) {
	if h.device != nil {
		_, _ = h.device.DenyDevice(st.UserCode)
	}
	h.renderDeviceMessage(c, http.StatusForbidden, "許可されていません", "このアカウントはこのサーバーへのログインを許可されていません。")
}

// DeviceTokenHandler is the RFC 8628 §3.4 polling endpoint; once approved it mints
// the CLI token and consumes the device_code so it cannot be redeemed twice.
func (h *Handler) DeviceTokenHandler(c *gin.Context) {
	if h.signer == nil || h.device == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device login not configured"})
		return
	}
	var body struct {
		DeviceCode string `json:"device_code"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.DeviceCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Enforce the advertised poll interval when the one-time guard is wired: a
	// second poll inside the window is answered slow_down (the guard entry, keyed by
	// device_code, self-evicts after one interval).
	if h.replay != nil {
		if firstUse, err := h.replay.Consume("devpoll:"+body.DeviceCode, deviceInterval); err == nil && !firstUse {
			c.JSON(http.StatusBadRequest, gin.H{"error": "slow_down"})
			return
		}
	}

	status, githubID, login, ok, err := h.device.PollDevice(body.DeviceCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "device store"})
		return
	}
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expired_token"})
		return
	}
	switch status {
	case auth.DeviceApproved:
		h.issueDeviceToken(c, body.DeviceCode, githubID, login)
	case auth.DeviceDenied:
		c.JSON(http.StatusBadRequest, gin.H{"error": "access_denied"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "authorization_pending"})
	}
}

// issueDeviceToken mints the CLI token for an approved authorization, re-checking
// the allowlist (fail-closed, in case it changed after approval) and consuming the
// device_code on success.
func (h *Handler) issueDeviceToken(c *gin.Context, deviceCode string, githubID int64, login string) {
	if !h.s.IsAdmin(githubID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "access_denied"})
		return
	}
	access, refresh, expiresIn, err := h.issueAccessRefresh(githubID, login, "cli")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	// Best-effort: the token is already minted, so a delete miss must not fail the
	// login. A replay is still blocked because a consumed record polls as expired.
	_ = h.device.ConsumeDevice(deviceCode)
	c.JSON(http.StatusOK, gin.H{
		"token":         access,
		"refresh_token": refresh,
		"expires_in":    expiresIn,
		"login":         login,
		"id":            githubID,
	})
}
