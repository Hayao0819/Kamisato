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
}

// AURConfig turns ayato into an aurweb-compatible host. When Enabled, ayato
// serves /rpc and redirects "<host>/<pkgbase>.git" for the PKGBUILD sources it
// has registered, falling through to the real AUR for everything else. Sources
// are registered by admins as external git URLs; ayato parses their .SRCINFO and
// keeps only the derived metadata (it stores no git state itself).
type AURConfig struct {
	Enabled bool `koanf:"enabled"`
	// Maintainer is the default synthetic maintainer label for registered
	// packages that do not carry their own.
	Maintainer string `koanf:"maintainer,omitempty"`
	// Upstream is the real-AUR fallback. Disabled Upstream makes ayato's AUR a
	// closed set limited to registered packages.
	Upstream UpstreamConfig `koanf:"upstream,omitempty"`
	// CatalogTTLMinutes bounds how long kayo treats a signed catalog as fresh
	// (the signed ExpiresAt); 0 uses a sane default. The signing seed is supplied
	// ONLY via the AYATO_AUR_SIGNING_SEED env var, never this config file.
	CatalogTTLMinutes int `koanf:"catalog_ttl_minutes,omitempty"`
}

// VerifyConfig is the trust root for cryptographic package-signature
// verification. Keyring points at a dedicated public-key file (armored or binary,
// kept separate from Build.GnupgHome, which holds the signing private key).
// TrustedKeys, when non-empty, pins which primary-key fingerprints are accepted.
type VerifyConfig struct {
	Keyring     string   `koanf:"keyring,omitempty"`
	TrustedKeys []string `koanf:"trusted_keys,omitempty"`
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

	// GitHub is the "Sign in with GitHub" OAuth2 client.
	GitHub GitHubOAuthConfig `koanf:"github,omitempty"`
	// PublicOrigin is the browser-facing SPA origin (e.g. https://repo.example.com):
	// the CORS allow-origin for cross-origin (bearer-mode) callers and the
	// postMessage target the web-bearer login posts the one-time code to. In the
	// same-origin (cookie/BFF) deployment it is also where the OAuth flow lands.
	PublicOrigin string `koanf:"public_origin,omitempty"`
	// SelfOrigin is ayato's OWN externally-reachable origin, used to build the
	// OAuth redirect_uri (the callback always hits ayato, never the static SPA).
	// Leave empty in the same-origin/BFF deployment, where it equals PublicOrigin;
	// set it to ayato's own URL (e.g. https://ayato.example.run.app) only when the
	// SPA is served cross-origin (bearer mode) so the callback resolves to ayato.
	SelfOrigin string `koanf:"self_origin,omitempty"`
	// BootstrapAdminGitHubID is seeded into the admin allowlist on first run
	// when the allowlist is empty.
	BootstrapAdminGitHubID int64 `koanf:"bootstrap_admin_github_id,omitempty"`
	// SessionCookieName is the web session cookie name (default "ayato_session").
	SessionCookieName string `koanf:"session_cookie_name,omitempty"`
	// SessionSecret holds one or more HMAC keys for the stateless auth signer
	// (sessions, CLI tokens, one-time codes, OAuth state). The first key signs;
	// ALL keys verify, so a key can be rotated by prepending a new one while
	// tokens minted under the old key keep verifying. Each key must be >= 32
	// bytes. Set via AYATO_AUTH_SESSION_SECRET.
	SessionSecret []string `koanf:"session_secret,omitempty"`
	// TrustedProxies are CIDRs/addresses (lumine) allowed to set X-Forwarded-*.
	// Empty now means trust NONE: ClientIP() falls back to the real peer and any
	// X-Forwarded-For is ignored. Set this to the fronting proxy's CIDR so the
	// per-IP rate-limit key reflects the real client rather than a spoofed header.
	TrustedProxies []string `koanf:"trusted_proxies,omitempty"`
	// CI holds non-interactive publish credentials (API key / GitHub OIDC) for CI
	// pipelines on the upload route, separate from the user admin allowlist.
	CI CIAuthConfig `koanf:"ci,omitempty"`
}

// CookieName returns the configured session cookie name or the default.
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

// Validate rejects an OAuth config that would let the redirect_uri or the
// session-cookie Secure flag be derived from spoofable request headers: when
// GitHub login is enabled, PublicOrigin is mandatory and must be an absolute
// http(s) origin without a path.
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
	// SelfOrigin is optional (defaults to PublicOrigin) but, when set, must be an
	// absolute origin without a path, same as PublicOrigin.
	if c.Auth.SelfOrigin != "" {
		su, serr := url.Parse(c.Auth.SelfOrigin)
		if serr != nil || (su.Scheme != "http" && su.Scheme != "https") || su.Host == "" {
			return fmt.Errorf("auth.self_origin must be an absolute http(s) URL with a host: %q", c.Auth.SelfOrigin)
		}
		if su.Path != "" && su.Path != "/" {
			return fmt.Errorf("auth.self_origin must be an origin without a path: %q", c.Auth.SelfOrigin)
		}
	}
	// The stateless signer is mandatory: without a secret there is no way to mint
	// or verify sessions/tokens. At least one key must be >= 32 bytes.
	if !hasUsableSessionSecret(c.Auth.SessionSecret) {
		return fmt.Errorf("auth.session_secret is required when GitHub login is enabled and each key must be at least 32 bytes")
	}
	// The per-IP rate-limit key is only trustworthy when a known proxy is the sole
	// X-Forwarded-For setter. Require trusted_proxies and reject trust-all entries
	// that would let any peer spoof their client IP.
	if len(c.Auth.TrustedProxies) == 0 {
		return fmt.Errorf("auth.trusted_proxies is required when GitHub login is enabled (set it to the fronting proxy's CIDR)")
	}
	for _, p := range c.Auth.TrustedProxies {
		if p == "*" {
			return fmt.Errorf("auth.trusted_proxies must not trust all peers (%q): set it to the fronting proxy's CIDR", p)
		}
		// Reject any-net CIDRs (prefix length 0) in EVERY spelling. String
		// matching specific forms like "0.0.0.0/0"/"::/0" misses equivalents such
		// as "0.0.0.0/00" or "0000:0000::/0", so parse and check the real prefix.
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

// hasUsableSessionSecret reports whether at least one configured secret meets the
// minimum HMAC key length (32 bytes).
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
