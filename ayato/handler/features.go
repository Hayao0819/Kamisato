package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

// FeaturesHandler advertises optional integrations and protocol capabilities so
// Lumine can hide unavailable UI and consume server-owned format manifests.
func (h *SystemHandler) FeaturesHandler(c *gin.Context) {
	features := domain.Features{
		BugReport:              h.bugReportEnabled,
		PackageArchiveSuffixes: pacmanpkg.SupportedArchiveSuffixes(),
	}
	if h.cfg != nil {
		features.Miko = h.cfg.Miko.URL != ""
		features.GitHubLogin = h.oauthEnabled != nil && h.oauthEnabled()
		features.RecaptchaSiteKey = h.cfg.Recaptcha.SiteKey
	}
	c.JSON(http.StatusOK, features)
}
