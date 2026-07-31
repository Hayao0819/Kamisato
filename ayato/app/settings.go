package app

import (
	"fmt"

	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/handler/bugreport"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/service"
)

func RepositorySettings(cfg *ayatoconfig.AyatoConfig) (repository.Settings, error) {
	if cfg == nil {
		return repository.Settings{}, fmt.Errorf("ayato config is nil")
	}
	catalog, err := cfg.RepositoryCatalog()
	if err != nil {
		return repository.Settings{}, fmt.Errorf("invalid repository catalog: %w", err)
	}
	settings := repository.Settings{
		Catalog:      catalog,
		SignDatabase: cfg.Sign.DB,
		Storage: repository.StorageSettings{
			Backend:  cfg.Store.StorageType,
			LocalDir: cfg.Store.LocalRepoDir,
			S3: repository.S3Settings{
				Bucket:          cfg.Store.AWSS3.Bucket,
				Region:          cfg.Store.AWSS3.Region,
				Endpoint:        cfg.Store.AWSS3.Endpoint,
				AccessKeyID:     cfg.Store.AWSS3.AccessKeyID,
				SecretAccessKey: cfg.Store.AWSS3.SecretAccessKey,
				SessionToken:    cfg.Store.AWSS3.SessionToken,
				UsePathStyle:    cfg.Store.AWSS3.UsePathStyle,
			},
		},
		KV: repository.KVSettings{
			Backend:    cfg.Store.DBType,
			BadgerPath: cfg.DbPath(),
			Cloudflare: repository.CloudflareKVSettings{
				AccountID: cfg.Store.CloudflareKV.AccountId,
				Token:     cfg.Store.CloudflareKV.Token,
				Namespace: cfg.Store.CloudflareKV.Namespace,
			},
		},
		Secrets: repository.SecretSettings{
			AgeIdentityFile: cfg.Secrets.AgeIdentityFile,
			Namespaces:      cfg.Secrets.Namespaces,
		},
	}
	if cfg.Store.DBType == "sql" || cfg.Store.DBType == "external" {
		settings.KV.SQLDriver = cfg.Store.SQL.Driver
		settings.KV.SQLDSN, err = cfg.Store.SQL.DSN()
		if err != nil {
			return repository.Settings{}, fmt.Errorf("configure SQL store: %w", err)
		}
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
			Provider: cfg.Recaptcha.Provider,
			SiteKey:  cfg.Recaptcha.SiteKey,
			Secret:   cfg.Recaptcha.Secret,
		},
		BugReport: bugreport.Config{
			Backends: cfg.BugReport.Backends,
			GitHub: bugreport.GitHubConfig{
				Repo:  cfg.BugReport.GitHub.Repo,
				Token: cfg.BugReport.GitHub.Token,
			},
			SMTP: bugreport.SMTPConfig{
				Host:         cfg.BugReport.SMTP.Host,
				Port:         cfg.BugReport.SMTP.Port,
				Username:     cfg.BugReport.SMTP.Username,
				Password:     cfg.BugReport.SMTP.Password,
				From:         cfg.BugReport.SMTP.From,
				To:           cfg.BugReport.SMTP.To,
				ToMaintainer: cfg.BugReport.SMTP.ToMaintainer,
			},
			Webhook: bugreport.WebhookConfig{URL: cfg.BugReport.Webhook.URL},
		},
	}
}

func ServiceSettings(cfg *ayatoconfig.AyatoConfig) service.Settings {
	if cfg == nil {
		return service.Settings{}
	}
	catalog, catalogErr := cfg.RepositoryCatalog()
	return service.Settings{
		Catalog:                    catalog,
		CatalogError:               catalogErr,
		RequireSign:                cfg.RequireSign,
		RequireBuildinfoProvenance: cfg.RequireBuildinfoProvenance,
		ExpectedBuildDir:           cfg.ExpectedBuildDir(),
		ProtectedNames:             cfg.ProtectedNames,
		MaxBatchPackages:           cfg.MaxBatchPackages,
		MaxPackageSize:             cfg.MaxSize,
		SignDatabase:               cfg.Sign.DB,
		VerificationKeyring:        cfg.Verify.Keyring,
		TrustedVerificationKeys:    cfg.Verify.TrustedKeys,
		MasterVerificationKeys:     cfg.Verify.MasterKeys,
	}
}
