// Package bugreport forwards user-submitted bug reports to external trackers
// (GitHub, SMTP, a generic webhook) behind Reporter, fanning each out to every
// configured backend. With no backend configured, reporting is off.
package bugreport

import (
	"context"
	"fmt"
)

// Report is one user-submitted bug report against a package.
type Report struct {
	Pkgname     string
	Pkgver      string
	Name        string
	Email       string
	Severity    string
	Description string
	// MaintainerEmail is the maintainer's address, resolved server-side and never
	// client-supplied. SMTP routing mails them when enabled.
	MaintainerEmail string
}

// Reporter forwards a report to a tracker and returns a link to the created entry.
type Reporter interface {
	Report(ctx context.Context, r Report) (url string, err error)
}

// Config is the bug-report backend configuration, mirrored here rather than
// imported from internal/conf to avoid an import cycle.
type Config struct {
	Backends []string
	GitHub   GitHubConfig
	SMTP     SMTPConfig
	Webhook  WebhookConfig
}

type GitHubConfig struct {
	Repo  string
	Token string
}

type SMTPConfig struct {
	Host         string
	Port         int
	Username     string
	Password     string
	From         string
	To           string
	ToMaintainer bool
}

type WebhookConfig struct {
	URL string
}

// New builds a Reporter from cfg.Backends (github/smtp/webhook). With no backends
// it returns (nil, nil), so callers treat reporting as off; unknown names error.
func New(cfg Config) (Reporter, error) {
	var reporters []Reporter
	for _, name := range cfg.Backends {
		switch name {
		case "github":
			r, err := newGitHub(cfg.GitHub)
			if err != nil {
				return nil, err
			}
			reporters = append(reporters, r)
		case "smtp":
			r, err := newSMTP(cfg.SMTP)
			if err != nil {
				return nil, err
			}
			reporters = append(reporters, r)
		case "webhook":
			r, err := newWebhook(cfg.Webhook)
			if err != nil {
				return nil, err
			}
			reporters = append(reporters, r)
		default:
			return nil, fmt.Errorf("bugreport: unknown backend %q", name)
		}
	}
	switch len(reporters) {
	case 0:
		return nil, nil
	case 1:
		return reporters[0], nil
	default:
		return &multiReporter{reporters: reporters}, nil
	}
}
