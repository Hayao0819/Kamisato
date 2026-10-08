package server

import (
	"os"

	"github.com/Hayao0819/Kamisato/ayato/bugreport"
	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// serviceSettings loads public verification material at the process boundary;
// the service parses the supplied bytes but never opens an operator path.
func serviceSettings(cfg *ayatoconfig.AyatoConfig) (service.Settings, error) {
	settings := ayatoconfig.ServiceSettings(cfg)
	if cfg != nil && cfg.Verify.Keyring != "" {
		data, err := os.ReadFile(cfg.Verify.Keyring)
		if err != nil {
			return service.Settings{}, errors.WrapErr(err, "load package-signature keyring")
		}
		settings.VerificationKeys = data
	}
	return settings, nil
}

func HandlerSettings(cfg *ayatoconfig.AyatoConfig) handler.Settings {
	if cfg == nil {
		return handler.Settings{}
	}
	catalog, _ := cfg.RepositoryCatalog()
	return handler.Settings{
		Catalog:                  catalog,
		DisableRedirectDownloads: !cfg.RedirectDownloadsEnabled(),
		MaxSize:                  cfg.MaxSize,
		MaxBatchPackages:         cfg.MaxBatchPackages,
		MaxBatchBytes:            cfg.MaxBatchBytes,
		RequireSign:              cfg.RequireSign,
		Miko: handler.MikoSettings{
			URL:    cfg.Miko.URL,
			APIKey: cfg.Miko.APIKey,
		},
		Auth: handler.AuthSettings{
			GitHubClientID:     cfg.Auth.GitHub.ClientID,
			GitHubClientSecret: cfg.Auth.GitHub.ClientSecret,
			PublicOrigin:       cfg.Auth.PublicOrigin,
			SelfOrigin:         cfg.Auth.SelfOrigin,
			CookieName:         cfg.Auth.CookieName(),
			AccessTokenTTL:     cfg.Auth.AccessTokenTTL(),
			RefreshTokenTTL:    cfg.Auth.RefreshTokenTTL(),
		},
		Mirror: handler.MirrorSettings{
			SelfURL:      cfg.Mirror.SelfURL,
			ServerPath:   cfg.Mirror.ServerPath(),
			UseRepoVar:   cfg.Mirror.UseRepoVar,
			AllCommented: cfg.Mirror.AllCommented,
		},
		Recaptcha: handler.RecaptchaSettings{
			SiteKey: cfg.Recaptcha.SiteKey,
		},
	}
}

func bugReportSettings(cfg ayatoconfig.BugReportConfig) bugreport.Config {
	return bugreport.Config{
		Backends: cfg.Backends,
		GitHub: bugreport.GitHubConfig{
			Repo:  cfg.GitHub.Repo,
			Token: cfg.GitHub.Token,
		},
		SMTP: bugreport.SMTPConfig{
			Host:         cfg.SMTP.Host,
			Port:         cfg.SMTP.Port,
			Username:     cfg.SMTP.Username,
			Password:     cfg.SMTP.Password,
			From:         cfg.SMTP.From,
			To:           cfg.SMTP.To,
			ToMaintainer: cfg.SMTP.ToMaintainer,
		},
		Webhook: bugreport.WebhookConfig{URL: cfg.Webhook.URL},
	}
}
