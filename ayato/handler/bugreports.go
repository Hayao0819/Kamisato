package handler

import (
	"net/http"
	"net/mail"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/handler/bugreport"
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
	Repo           string `json:"repo"` // optional, used to resolve the maintainer
	Arch           string `json:"arch"` // optional, used to resolve the maintainer
	Name           string `json:"name"`
	Email          string `json:"email"`
	Severity       string `json:"severity"`
	Description    string `json:"description"`
	RecaptchaToken string `json:"recaptcha_token"`
}

// SubmitBugReportHandler forwards a report to the configured tracker.
func (h *BugReportHandler) SubmitBugReportHandler(c *gin.Context) {
	if h.reporter == nil {
		respondError(c, http.StatusServiceUnavailable, "bug reporting is not configured")
		return
	}
	var req bugReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Description == "" || req.Pkgname == "" {
		respondError(c, http.StatusBadRequest, "package name and description are required")
		return
	}
	if len(req.Description) > maxBugDescLen ||
		len(req.Pkgname) > maxBugFieldLen || len(req.Pkgver) > maxBugFieldLen ||
		len(req.Name) > maxBugFieldLen || len(req.Email) > maxBugFieldLen ||
		len(req.Repo) > maxBugFieldLen || len(req.Arch) > maxBugFieldLen {
		respondError(c, http.StatusBadRequest, "a field exceeds its maximum length")
		return
	}
	severity := req.Severity
	if severity == "" {
		severity = "medium"
	} else if !bugSeverities[severity] {
		respondError(c, http.StatusBadRequest, "invalid severity")
		return
	}

	if h.recaptcha != nil {
		if err := h.recaptcha.Verify(c.Request.Context(), req.RecaptchaToken, c.ClientIP()); err != nil {
			respondError(c, http.StatusBadRequest, "reCAPTCHA verification failed")
			return
		}
	}

	url, err := h.reporter.Report(c.Request.Context(), bugreport.Report{
		Pkgname:         req.Pkgname,
		Pkgver:          req.Pkgver,
		Name:            req.Name,
		Email:           req.Email,
		Severity:        severity,
		Description:     req.Description,
		MaintainerEmail: h.maintainerEmail(req.Repo, req.Arch, req.Pkgname),
	})
	if err != nil {
		respondLoggedError(c, http.StatusBadGateway, "file bug report", "failed to file the bug report", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"url": url})
}

// The maintainer address is resolved from the stored PKGINFO Packager, never from
// the request, so a reporter cannot spoof who gets mailed.
func (h *BugReportHandler) maintainerEmail(repo, arch, pkgname string) string {
	if repo == "" || arch == "" || pkgname == "" || h.reader == nil {
		return ""
	}
	detail, err := h.reader.PkgDetail(repo, arch, pkgname)
	if err != nil || detail == nil {
		return ""
	}
	return parsePackagerEmail(detail.Packager)
}

func parsePackagerEmail(packager string) string {
	packager = strings.TrimSpace(packager)
	if packager == "" {
		return ""
	}
	if addr, err := mail.ParseAddress(packager); err == nil {
		return addr.Address
	}
	if i := strings.IndexByte(packager, '<'); i >= 0 {
		if j := strings.IndexByte(packager[i+1:], '>'); j >= 0 {
			if addr, err := mail.ParseAddress(packager[i+1 : i+1+j]); err == nil {
				return addr.Address
			}
		}
	}
	return ""
}
