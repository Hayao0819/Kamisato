package conf

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"path"

	"github.com/spf13/pflag"
)

type AyatoConfig struct {
	Debug       bool            `koanf:"debug"`
	RequireSign bool            `koanf:"require_sign"`
	Port        int             `koanf:"port"`
	MaxSize     int             `koanf:"maxsize"`
	Repos       []BinRepoConfig `koanf:"repos"`
	Auth        AuthConfig      `koanf:"auth"`
	Store       StoreConfig     `koanf:"store"`
	Build       BuildConfig     `koanf:"build"`
	Miko        MikoUpstream    `koanf:"miko"`
	Verify      VerifyConfig    `koanf:"verify"`
	AUR         AURConfig       `koanf:"aur"`
	// RedirectDownloads, unset by default, answers a file download with a 302 to a
	// presigned object URL whenever the blob backend can presign (S3), so the bytes
	// go client<->object-store directly and skip ayato's egress (Cloud Run bills it).
	// Set it to false to force every download to stream through ayato; a backend that
	// cannot presign (localfs) always streams regardless.
	RedirectDownloads *bool `koanf:"redirect_downloads,omitempty"`
}

// RedirectDownloadsEnabled reports whether downloads should be redirected to a
// presigned URL. It defaults to true so egress offload is automatic; the redirect
// still only happens when the backend can actually presign.
func (c *AyatoConfig) RedirectDownloadsEnabled() bool {
	return c.RedirectDownloads == nil || *c.RedirectDownloads
}

// AURConfig makes ayato an aurweb-compatible host: when Enabled it serves /rpc and
// git redirects for registered PKGBUILD sources, falling through to the real AUR
// otherwise, and keeps only the derived .SRCINFO metadata (no git state).
type AURConfig struct {
	Enabled bool `koanf:"enabled"`
	// Maintainer is the default synthetic maintainer for registered packages lacking one.
	Maintainer string `koanf:"maintainer,omitempty"`
	// Upstream is the real-AUR fallback. Disabled Upstream makes ayato's AUR a
	// closed set limited to registered packages.
	Upstream UpstreamConfig `koanf:"upstream,omitempty"`
	// CatalogTTLMinutes bounds how long kayo treats a signed catalog as fresh; 0 uses
	// a default. The signing seed comes ONLY from AYATO_AUR_SIGNING_SEED, never config.
	CatalogTTLMinutes int `koanf:"catalog_ttl_minutes,omitempty"`
}

// VerifyConfig is the trust root for package-signature verification. Keyring is a
// public-key file, separate from Build.GnupgHome (the signing private key).
// TrustedKeys, when set, pins which primary-key fingerprints are accepted.
type VerifyConfig struct {
	Keyring     string   `koanf:"keyring,omitempty"`
	TrustedKeys []string `koanf:"trusted_keys,omitempty"`
	// MasterKeys are armored master public keys. A worker key certified by one of
	// these is accepted once registered, so adding a worker needs no ayato change.
	MasterKeys []string `koanf:"master_keys,omitempty"`
}

// GitHubOAuthConfig is the confidential OAuth2 client used for "Sign in with
// GitHub". Empty ClientID disables the GitHub login flow.
type GitHubOAuthConfig struct {
	ClientID     string `koanf:"client_id,omitempty"`
	ClientSecret string `koanf:"client_secret,omitempty"`
}

// MikoUpstream is the internal build server ayato proxies build/job requests to.
type MikoUpstream struct {
	URL    string `koanf:"url"`     // internal base URL, e.g. http://miko:8081
	APIKey string `koanf:"api_key"` // shared secret sent to miko on every proxied call
}

type BuildConfig struct {
	Image     string `koanf:"image"`      // Docker image (default: "archlinux:latest")
	Timeout   int    `koanf:"timeout"`    // Build timeout in minutes (default: 30)
	GnupgHome string `koanf:"gnupg_home"` // GPG home directory for signing
}

type AuthConfig struct {
	Username string `koanf:"username,omitempty"`
	Password string `koanf:"password,omitempty"`

	GitHub GitHubOAuthConfig `koanf:"github,omitempty"`
	// PublicOrigin is the browser-facing SPA origin (e.g. https://repo.example.com):
	// the CORS allow-origin for bearer-mode callers and the postMessage target for the
	// one-time login code. In the same-origin/BFF deployment the OAuth flow lands here.
	PublicOrigin string `koanf:"public_origin,omitempty"`
	// SelfOrigin is ayato's OWN externally-reachable origin for building the OAuth
	// redirect_uri. Leave empty in same-origin/BFF (equals PublicOrigin); set it only
	// when the SPA is served cross-origin (bearer mode) so the callback resolves to
	// ayato.
	SelfOrigin string `koanf:"self_origin,omitempty"`
	// BootstrapAdminGitHubID is seeded into the admin allowlist on first run
	// when the allowlist is empty.
	BootstrapAdminGitHubID int64 `koanf:"bootstrap_admin_github_id,omitempty"`
	// SessionCookieName is the web session cookie name (default "ayato_session").
	SessionCookieName string `koanf:"session_cookie_name,omitempty"`
	// SessionSecret holds HMAC keys for the stateless auth signer. The first signs;
	// all verify, so a key rotates by prepending a new one. Each must be >= 32 bytes.
	// Set via AYATO_AUTH_SESSION_SECRET.
	SessionSecret []string `koanf:"session_secret,omitempty"`
	// TrustedProxies are CIDRs/addresses allowed to set X-Forwarded-*. Empty trusts
	// none, so ClientIP() ignores X-Forwarded-For and uses the real peer. Set the
	// fronting proxy's CIDR so the per-IP rate-limit key reflects the real client.
	TrustedProxies []string `koanf:"trusted_proxies,omitempty"`
	// CI holds non-interactive publish credentials (API key / GitHub OIDC) for CI
	// pipelines on the upload route, separate from the user admin allowlist.
	CI CIAuthConfig `koanf:"ci,omitempty"`
}

func (a AuthConfig) CookieName() string {
	if a.SessionCookieName != "" {
		return a.SessionCookieName
	}
	return "ayato_session"
}

type BinRepoConfig struct {
	Name   string   `koanf:"name"`
	Arches []string `koanf:"arches"`
	// TrustedKeys optionally layers per-repo fingerprint pinning on top of the
	// global Verify.TrustedKeys allowlist. The global keyring is the baseline.
	TrustedKeys []string `koanf:"trusted_keys,omitempty"`
}

func LoadAyatoConfig(flags *pflag.FlagSet, configFile string) (*AyatoConfig, error) {
	if err := LoadEnv(); err != nil {
		slog.Error("Failed to load env", "error", err)
	}

	dirs := commonConfigDirs()
	files := []string{}
	if configFile != "" {
		files = append(files, configFile)
	} else {
		files = []string{"ayato_config.json", "ayato_config.toml", "ayato_config.yaml"}
	}

	cfg, err := loadConfig[AyatoConfig](
		dirs,
		files,
		flags,
		"AYATO",
	)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate rejects an OAuth config whose redirect_uri or session-cookie Secure flag
// could be derived from spoofable request headers: with GitHub login on,
// PublicOrigin is mandatory and must be an absolute http(s) origin without a path.
func (c *AyatoConfig) Validate() error {
	if err := c.Auth.CI.validate(); err != nil {
		return err
	}
	githubEnabled := c.Auth.GitHub.ClientID != "" || c.Auth.GitHub.ClientSecret != ""
	if !githubEnabled {
		return nil
	}
	if c.Auth.GitHub.ClientID == "" || c.Auth.GitHub.ClientSecret == "" {
		return fmt.Errorf("auth.github: client_id and client_secret are both required when GitHub login is enabled")
	}
	if c.Auth.PublicOrigin == "" {
		return fmt.Errorf("auth.public_origin is required when GitHub login is enabled (e.g. https://repo.example.com)")
	}
	u, err := url.Parse(c.Auth.PublicOrigin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("auth.public_origin must be an absolute http(s) URL with a host: %q", c.Auth.PublicOrigin)
	}
	if u.Path != "" && u.Path != "/" {
		return fmt.Errorf("auth.public_origin must be an origin without a path: %q", c.Auth.PublicOrigin)
	}
	if c.Auth.SelfOrigin != "" {
		su, serr := url.Parse(c.Auth.SelfOrigin)
		if serr != nil || (su.Scheme != "http" && su.Scheme != "https") || su.Host == "" {
			return fmt.Errorf("auth.self_origin must be an absolute http(s) URL with a host: %q", c.Auth.SelfOrigin)
		}
		if su.Path != "" && su.Path != "/" {
			return fmt.Errorf("auth.self_origin must be an origin without a path: %q", c.Auth.SelfOrigin)
		}
	}
	// The stateless signer is mandatory: no secret means no sessions/tokens.
	if !hasUsableSessionSecret(c.Auth.SessionSecret) {
		return fmt.Errorf("auth.session_secret is required when GitHub login is enabled and each key must be at least 32 bytes")
	}
	// The per-IP rate-limit key is only trustworthy when a known proxy is the sole
	// X-Forwarded-For setter, so require trusted_proxies and reject trust-all.
	if len(c.Auth.TrustedProxies) == 0 {
		return fmt.Errorf("auth.trusted_proxies is required when GitHub login is enabled (set it to the fronting proxy's CIDR)")
	}
	for _, p := range c.Auth.TrustedProxies {
		if p == "*" {
			return fmt.Errorf("auth.trusted_proxies must not trust all peers (%q): set it to the fronting proxy's CIDR", p)
		}
		// Reject any-net CIDRs (prefix length 0) in every spelling: string-matching
		// "0.0.0.0/0"/"::/0" misses equivalents like "0.0.0.0/00", so parse instead.
		if _, ipnet, err := net.ParseCIDR(p); err == nil {
			if ones, _ := ipnet.Mask.Size(); ones == 0 {
				return fmt.Errorf("auth.trusted_proxies must not trust an any-net CIDR (%q): set it to the fronting proxy's CIDR", p)
			}
		} else if net.ParseIP(p) == nil {
			return fmt.Errorf("auth.trusted_proxies entry %q is not a valid IP or CIDR", p)
		}
	}
	return nil
}

func hasUsableSessionSecret(secrets []string) bool {
	for _, s := range secrets {
		if len(s) >= 32 {
			return true
		}
	}
	return false
}

func (c *AyatoConfig) DbPath() string {
	return path.Join(c.Store.BadgerDB, "kv-db")
}

func (c *AyatoConfig) RepoNames() []string {
	repoNames := make([]string, len(c.Repos))
	for i, repo := range c.Repos {
		repoNames[i] = repo.Name
	}
	return repoNames
}

func (c *AyatoConfig) Repo(name string) *BinRepoConfig {
	for _, repo := range c.Repos {
		if repo.Name == name {
			return &repo
		}
	}
	return nil
}
