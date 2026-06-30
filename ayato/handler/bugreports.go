package handler

import (
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/bugreport"
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
)

// Field caps so a forwarded report cannot abuse the upstream tracker.
const (
	maxBugFieldLen = 256
	maxBugDescLen  = 8 << 10
)

var bugSeverities = map[string]bool{"critical": true, "high": true, "medium": true, "low": true}

type bugReportRequest struct {
	Pkgname        string `json:"pkgname"`
	Pkgver         string `json:"pkgver"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Severity       string `json:"severity"`
	Description    string `json:"description"`
	RecaptchaToken string `json:"recaptcha_token"`
}

// FeaturesHandler advertises which optional features are configured so the web
// UI can hide what is not available (the report button, miko-backed build/jobs
// views, the GitHub login). recaptcha_site_key is non-empty only when the bug
// form must render a reCAPTCHA widget.
func (h *Handler) FeaturesHandler(c *gin.Context) {
	feat := gin.H{
		"bug_report":         h.reporter != nil,
		"miko":               false,
		"github_login":       false,
		"recaptcha_site_key": "",
	}
	if h.cfg != nil {
		feat["miko"] = h.cfg.Miko.URL != ""
		feat["github_login"] = h.oauthEnabled()
		feat["recaptcha_site_key"] = h.cfg.Recaptcha.SiteKey
	}
	c.JSON(http.StatusOK, feat)
}

// SubmitBugReportHandler forwards a report to the configured tracker.
func (h *Handler) SubmitBugReportHandler(c *gin.Context) {
	if h.reporter == nil {
		c.JSON(http.StatusServiceUnavailable, domain.APIError{Message: "bug reporting is not configured"})
		return
	}
	var req bugReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.APIError{Message: "invalid request", Reason: err.Error()})
		return
	}
	if req.Description == "" || req.Pkgname == "" {
		c.JSON(http.StatusBadRequest, domain.APIError{Message: "package name and description are required"})
		return
	}
	if len(req.Description) > maxBugDescLen ||
		len(req.Pkgname) > maxBugFieldLen || len(req.Pkgver) > maxBugFieldLen ||
		len(req.Name) > maxBugFieldLen || len(req.Email) > maxBugFieldLen {
		c.JSON(http.StatusBadRequest, domain.APIError{Message: "a field exceeds its maximum length"})
		return
	}
	severity := req.Severity
	if severity == "" {
		severity = "medium"
	} else if !bugSeverities[severity] {
		c.JSON(http.StatusBadRequest, domain.APIError{Message: "invalid severity"})
		return
	}

	if h.recaptcha != nil {
		if err := h.recaptcha.Verify(c.Request.Context(), req.RecaptchaToken, c.ClientIP()); err != nil {
			c.JSON(http.StatusBadRequest, domain.APIError{Message: "reCAPTCHA verification failed", Reason: err.Error()})
			return
		}
	}

	url, err := h.reporter.Report(c.Request.Context(), bugreport.Report{
		Pkgname:     req.Pkgname,
		Pkgver:      req.Pkgver,
		Name:        req.Name,
		Email:       req.Email,
		Severity:    severity,
		Description: req.Description,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, domain.APIError{Message: "failed to file the bug report", Reason: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"url": url})
}
