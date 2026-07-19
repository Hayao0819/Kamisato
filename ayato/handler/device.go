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

var deviceMessageTmpl = template.Must(template.New("devicemsg").Parse(
	`<!doctype html><meta charset="utf-8"><title>Device login</title>
<body style="font-family:sans-serif;max-width:32rem;margin:4rem auto">
<h1>{{.Title}}</h1><p>{{.Body}}</p></body>`))

func renderDevicePage(c *gin.Context, status int, tmpl *template.Template, data any) {
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(c.Writer, data)
}

func (h *AuthHandler) renderDeviceForm(c *gin.Context, status int, userCode, errMsg string) {
	renderDevicePage(c, status, deviceFormTmpl, struct{ UserCode, Error string }{userCode, errMsg})
}

func (h *AuthHandler) renderDeviceMessage(c *gin.Context, status int, title, body string) {
	renderDevicePage(c, status, deviceMessageTmpl, struct{ Title, Body string }{title, body})
}

func normalizeUserCode(value string) string {
	var builder strings.Builder
	for _, character := range strings.ToUpper(value) {
		if character >= 'A' && character <= 'Z' {
			builder.WriteRune(character)
		}
	}
	letters := builder.String()
	if len(letters) == 8 {
		return letters[:4] + "-" + letters[4:]
	}
	return letters
}

// DeviceCodeHandler issues a device_code + user_code pair (RFC 8628 §3.2).
func (h *AuthHandler) DeviceCodeHandler(c *gin.Context) {
	if !h.oauthConfigured() || h.device == nil {
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

func (h *AuthHandler) DeviceVerifyHandler(c *gin.Context) {
	if !h.oauthConfigured() || h.device == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device login not configured"})
		return
	}
	h.renderDeviceForm(c, http.StatusOK, normalizeUserCode(c.Query("user_code")), "")
}

func (h *AuthHandler) DeviceApproveHandler(c *gin.Context) {
	if !h.oauthConfigured() || h.device == nil {
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

func (h *AuthHandler) finishDeviceLogin(c *gin.Context, st *auth.Claims, user githubUser) {
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

func (h *AuthHandler) denyDeviceLogin(c *gin.Context, st *auth.Claims) {
	if h.device != nil {
		_, _ = h.device.DenyDevice(st.UserCode)
	}
	h.renderDeviceMessage(c, http.StatusForbidden, "許可されていません", "このアカウントはこのサーバーへのログインを許可されていません。")
}
