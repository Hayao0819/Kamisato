package conf

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/spf13/pflag"

	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/limits"
)

type AyatoConfig struct {
	Debug       bool `koanf:"debug"`
	RequireSign bool `koanf:"require_sign"`
	// RequireBuildinfoProvenance gates ingest on the uploaded package's .BUILDINFO
	// builddir matching BuildinfoBuildDir (miko's sandbox root). It is off by default
	// so a deployment not fronting miko builders keeps accepting packages built
	// elsewhere; turn it on to reject anything not built in the clean sandbox (a
	// builder-infection signal). A missing .BUILDINFO fails closed.
	RequireBuildinfoProvenance bool `koanf:"require_buildinfo_provenance"`
	// BuildinfoBuildDir is the builddir the provenance gate requires; empty means
	// the default "/build" (miko's sandbox root).
	BuildinfoBuildDir string `koanf:"buildinfo_builddir,omitempty"`
	// ProtectedNames is an opt-in supply-chain guard: an upload whose pkgname — or
	// any of its provides/replaces/groups — collides with one of these names is
	// rejected, so an AUR-style package cannot masquerade as (shadow) an official
	// package such as "pacman" or "glibc". Empty disables the guard.
	ProtectedNames []string `koanf:"protected_names,omitempty"`
	Port           int      `koanf:"port"`
	// MaxSize is the maximum package size in bytes. Zero selects the shared
	// default also used by miko.
	MaxSize int `koanf:"max_size"`
	// Zero uses the shared batch defaults.
	MaxBatchPackages int   `koanf:"max_batch_packages"`
	MaxBatchBytes    int64 `koanf:"max_batch_bytes"`
	// DefaultArches is the arch set a repo with no explicit Arches serves, so the
	// per-repo declaration can be omitted when every repo shares one arch list.
	// Empty leaves per-repo Arches as the only declared source.
	DefaultArches []string           `koanf:"default_arches,omitempty"`
	Repos         []BinRepoConfig    `koanf:"repos"`
	Auth          AuthConfig         `koanf:"auth"`
	Store         StoreConfig        `koanf:"store"`
	Build         BuildServiceConfig `koanf:"build"`
	Miko          MikoUpstream       `koanf:"miko"`
	Verify        VerifyConfig       `koanf:"verify"`
	AUR           AURConfig          `koanf:"aur"`
	BugReport     BugReportConfig    `koanf:"bug_report"`
	Recaptcha     RecaptchaConfig    `koanf:"recaptcha"`
	Sign          SignConfig         `koanf:"sign"`
	Secrets       SecretsConfig      `koanf:"secrets"`
	// Mirror configures the pacman mirrorlist served at /repo/<repo>/mirrorlist.
	Mirror MirrorConfig `koanf:"mirror"`
	// RedirectDownloads, unset by default, answers a file download with a 302 to a
	// presigned object URL whenever the blob backend can presign (S3), so the bytes
	// go client<->object-store directly and skip ayato's egress (Cloud Run bills it).
	// Set it to false to force every download to stream through ayato; a backend that
	// cannot presign (localfs) always streams regardless.
	RedirectDownloads *bool `koanf:"redirect_downloads"`
}

// MirrorConfig configures the pacman mirrorlist ayato generates at
// /repo/<repo>/mirrorlist; it currently advertises only this instance.
type MirrorConfig struct {
	// SelfURL is this instance's public base URL (e.g. https://repo.example). The
	// Server line becomes <SelfURL><RepoPath>/<repo>/$arch. Empty falls back to
	// auth.self_origin, then auth.public_origin, then the request host.
	SelfURL string `koanf:"self_url,omitempty"`
	// RepoPath is the path prefix repos are served under; empty means "/repo".
	RepoPath string `koanf:"repo_path,omitempty"`
	// UseRepoVar writes the pacman $repo variable instead of the concrete repo
	// name into the Server path. Default (false) bakes the repo name in.
	UseRepoVar bool `koanf:"use_repo_var,omitempty"`
	// AllCommented ships every Server line commented out (official-Arch style) so
	// the user opts in per mirror. Default (false) leaves this instance active.
	AllCommented bool `koanf:"all_commented,omitempty"`
}

// ServerPath returns the Server-line path prefix, normalized to a single leading
// slash and no trailing/interior redundant slashes (default "/repo").
func (m MirrorConfig) ServerPath() string {
	p := strings.Trim(m.RepoPath, "/")
	if p == "" {
		return "/repo"
	}
	return path.Clean("/" + p)
}

// SignConfig toggles optional signing features. Only the toggle lives in config;
// the private keys come from the environment (never the config file), matching
// the AUR catalog seed.
type SignConfig struct {
	// DB enables signing the pacman repo database, producing <repo>.db.tar.gz.sig
	// and <repo>.files.tar.gz.sig. The armored private key comes from
	// AYATO_DB_SIGNING_KEY (optionally unlocked with AYATO_DB_SIGNING_PASSPHRASE).
	DB bool `koanf:"db,omitempty"`
}

// SecretsConfig enables at-rest encryption of sensitive kv records (the admin
// allowlist). It is opt-in: with no age identity configured the records stay in
// plaintext exactly as before, and enabling it later reads existing plaintext back
// transparently. The age X25519 secret key comes from the environment
// (AYATO_SECRETS_AGE_IDENTITY) or a file, never the config value itself — matching
// how the other private keys are sourced.
type SecretsConfig struct {
	// AgeIdentityFile points at a file holding the age X25519 secret key
	// ("AGE-SECRET-KEY-1..."), e.g. a mounted secret. AYATO_SECRETS_AGE_IDENTITY
	// takes precedence over it. Empty (and no env) leaves encryption off.
	AgeIdentityFile string `koanf:"age_identity_file,omitempty"`
	// Namespaces overrides which kv namespaces are encrypted. Empty uses the default
	// sensitive set (the admin allowlist).
	Namespaces []string `koanf:"namespaces,omitempty"`
}

// ExpectedBuildDir returns the builddir the .BUILDINFO provenance gate requires,
// defaulting to miko's "/build" sandbox root when unset.
func (c *AyatoConfig) ExpectedBuildDir() string {
	if c.BuildinfoBuildDir != "" {
		return c.BuildinfoBuildDir
	}
	return "/build"
}

// RedirectDownloadsEnabled reports whether downloads redirect to a presigned URL
// (default true); the redirect still only happens when the backend can presign.
func (c *AyatoConfig) RedirectDownloadsEnabled() bool {
	return c.RedirectDownloads == nil || *c.RedirectDownloads
}

// BugReportConfig selects the external trackers bug reports are forwarded to.
// Backends lists every enabled sink; an empty list disables the feature (the UI
// hides its report button). A report fans out to all of them.
type BugReportConfig struct {
	Backends []string `koanf:"backends"` // any of "github", "smtp", "webhook"; empty disables reporting
	GitHub   struct {
		Repo  string `koanf:"repo"`  // "owner/name"
		Token string `koanf:"token"` // token allowed to open issues
	} `koanf:"github"`
	SMTP struct {
		Host         string `koanf:"host"`
		Port         int    `koanf:"port"`
		Username     string `koanf:"username"`
		Password     string `koanf:"password"`
		From         string `koanf:"from"`
		To           string `koanf:"to"`
		ToMaintainer bool   `koanf:"to_maintainer"` // also mail the package maintainer when known
	} `koanf:"smtp"`
	Webhook struct {
		URL string `koanf:"url"`
	} `koanf:"webhook"`
}

// RecaptchaConfig enables a CAPTCHA on the bug-report form. SiteKey is public
// (handed to the browser); an empty Secret disables verification. Provider selects
// the backend: "recaptcha" (Google reCAPTCHA v2, the default) or "turnstile"
// (Cloudflare Turnstile), which share the same siteverify contract.
type RecaptchaConfig struct {
	Provider string `koanf:"provider"` // "recaptcha" (default) or "turnstile"
	SiteKey  string `koanf:"site_key"`
	Secret   string `koanf:"secret"`
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
	// RateLimitPerDay caps aurweb /rpc requests per client per day (aurweb's own
	// limit). Unset applies the default; an explicit 0 disables throttling. It is a
	// pointer so unset and an intentional 0 are distinguishable, like RedirectDownloads.
	RateLimitPerDay *int `koanf:"rate_limit_per_day,omitempty"`
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

// BuildServiceConfig keeps orchestration policy separate from builder.HostConfig.
type BuildServiceConfig struct {
	// Image and Timeout preserve the legacy Miko schema.
	Image     string `koanf:"image"`
	Timeout   int    `koanf:"timeout"`
	GnupgHome string `koanf:"gnupg_home"` // GPG home directory for signing
	// ExtraRepos preserves the legacy Miko schema.
	ExtraRepos []builder.PacmanRepository `koanf:"extra_repos"`
	// ResolveAURDeps, when set, makes miko resolve a target's unbuilt AUR
	// dependencies (from its .SRCINFO), build each in dependency order and publish
	// it to ayato before building the target. Opt-in; the target repo is exposed to
	// the build automatically so the freshly published dependencies resolve.
	ResolveAURDeps bool `koanf:"resolve_aur_deps"`
	// AURRPCURL overrides the aurweb RPC endpoint used to resolve AUR dependencies.
	// Empty uses the canonical AUR.
	AURRPCURL string `koanf:"aur_rpc_url"`
}

type AuthConfig struct {
	// Username and Password are deprecated Basic-auth settings.
	Username string            `koanf:"username,omitempty"`
	Password string            `koanf:"password,omitempty"`
	GitHub   GitHubOAuthConfig `koanf:"github,omitempty"`
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
	// SessionCookieName is the web session cookie name (default
	// "__Host-ayato_session"). The __Host- prefix pins the cookie to the exact
	// host over HTTPS and requires Secure + Path=/ + no Domain (all emitted by
	// setSessionCookie). Plain-HTTP localhost dev cannot set a __Host- cookie, so
	// serve HTTPS or override this via AYATO_AUTH_SESSION_COOKIE_NAME.
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
	// AllowLegacySignerBasic enables the signer-registration migration bridge.
	AllowLegacySignerBasic bool `koanf:"allow_legacy_signer_basic,omitempty"`
	// AccessTokenTTLMinutes bounds a freshly issued CLI access token's lifetime.
	// Short by default (1h) so a leaked access token is a small window; the client
	// silently trades a refresh token for a new one. Unset/<=0 uses the default. A
	// pre-existing long-lived token (no refresh) keeps working until its own expiry.
	AccessTokenTTLMinutes int `koanf:"access_token_ttl_minutes,omitempty"`
	// RefreshTokenTTLDays bounds the refresh token's lifetime, i.e. how long a CLI
	// session survives without a re-login. Unset/<=0 uses the default (30 days).
	RefreshTokenTTLDays int `koanf:"refresh_token_ttl_days,omitempty"`
}

func (a AuthConfig) CookieName() string {
	if a.SessionCookieName != "" {
		return a.SessionCookieName
	}
	return "__Host-ayato_session"
}

// AccessTokenTTL is the lifetime of a newly minted CLI access token (default 1h).
func (a AuthConfig) AccessTokenTTL() time.Duration {
	if a.AccessTokenTTLMinutes > 0 {
		return time.Duration(a.AccessTokenTTLMinutes) * time.Minute
	}
	return time.Hour
}

// RefreshTokenTTL is the lifetime of a newly minted refresh token (default 30d).
func (a AuthConfig) RefreshTokenTTL() time.Duration {
	if a.RefreshTokenTTLDays > 0 {
		return time.Duration(a.RefreshTokenTTLDays) * 24 * time.Hour
	}
	return 30 * 24 * time.Hour
}

type BinRepoConfig struct {
	Name string `koanf:"name"`
	// Arches is the architectures this repo serves. An upload whose package arch is
	// outside this set is rejected unless AllowNewArch, so a mislabeled package can
	// never silently add an arch. Empty inherits DefaultArches.
	Arches []string `koanf:"arches,omitempty"`
	// AllowNewArch accepts an upload for an arch not in Arches, letting the repo grow
	// a new arch on demand. Off by default so the served arch set stays what is
	// declared.
	AllowNewArch bool `koanf:"allow_new_arch,omitempty"`
	// TrustedKeys optionally layers per-repo fingerprint pinning on top of the
	// global Verify.TrustedKeys allowlist. The global keyring is the baseline.
	TrustedKeys []string `koanf:"trusted_keys,omitempty"`
	// Tiered exposes three pacman repos for this logical repo — <name>-staging,
	// <name>-testing and <name> (stable) — that a built package flows through by
	// explicit promotion. Off by default, so the repo stays a single <name> repo
	// and behaves exactly as before.
	Tiered bool `koanf:"tiered,omitempty"`
	// PromotionKeepInSource keeps a promoted package published in its source tier
	// instead of moving it to the next one. Default (false) is a move.
	PromotionKeepInSource bool `koanf:"promotion_keep_in_source,omitempty"`
	// Upstream layers this repo on top of a referenced pacman repo database: the
	// served .db/.files becomes the merge of the upstream database with
	// locally-published packages, local shadowing upstream on a name collision.
	// Empty disables layering, so the repo behaves exactly as a plain repo.
	Upstream UpstreamRepoConfig `koanf:"upstream,omitempty"`
}

// UpstreamRepoConfig configures the upstream a repo layers local packages on top
// of (the CachyOS overlay model).
type UpstreamRepoConfig struct {
	// DBURL is the upstream .db URL, e.g. an official Arch mirror path
	// "https://mirror/extra/os/$arch/extra.db" or another ayato. A "$arch"
	// placeholder is substituted per architecture. Empty disables layering.
	DBURL string `koanf:"db_url,omitempty"`
	// FilesURL is the matching .files URL for the merged files database; empty
	// derives it from DBURL by swapping ".db" for ".files".
	FilesURL string `koanf:"files_url,omitempty"`
}

func LoadAyatoConfig(flags *pflag.FlagSet, configFile string) (*AyatoConfig, error) {
	loadDotEnv()
	return LoadTyped[AyatoConfig](
		commonConfigDirs(),
		configFileNames(configFile, "ayato_config"),
		flags,
		"AYATO",
		(*AyatoConfig).applyDefaults,
	)
}

// DefaultPort is the listen port used when neither an explicit port
// (AYATO_PORT / config "port") nor Cloud Run's injected PORT is set.
const DefaultPort = 8080

// applyDefaults fills in load-time defaults that depend on the ambient
// environment, keeping cfg.Port a valid port so the server never binds ":0".
func (c *AyatoConfig) applyDefaults() {
	c.Port = resolvePort(c.Port, os.Getenv("PORT"))
}

// resolvePort picks the listen port by precedence: an explicit configured port
// (AYATO_PORT / config "port") wins; otherwise Cloud Run's injected PORT; else
// DefaultPort. A non-positive or unparseable value counts as unset so the result
// is always a usable port.
func resolvePort(configured int, portEnv string) int {
	if configured > 0 {
		return configured
	}
	if portEnv != "" {
		if n, err := strconv.Atoi(portEnv); err == nil && n > 0 {
			return n
		}
	}
	return DefaultPort
}

func validateHTTPOrigin(name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s must be an absolute http(s) URL with a host: %q", name, raw)
	}
	if u.Path != "" && u.Path != "/" {
		return fmt.Errorf("%s must be an origin without a path: %q", name, raw)
	}
	return nil
}

// Validate rejects an OAuth config whose redirect_uri or session-cookie Secure flag
// could be derived from spoofable request headers: with GitHub login on,
// PublicOrigin is mandatory and must be an absolute http(s) origin without a path.
func (c *AyatoConfig) Validate() error {
	if err := c.Store.Validate(); err != nil {
		return err
	}
	if _, err := c.RepositoryCatalog(); err != nil {
		return fmt.Errorf("repos: %w", err)
	}
	if err := c.Store.checkStateless(UnderCloudRun()); err != nil {
		return err
	}
	if err := c.Auth.CI.validate(); err != nil {
		return err
	}
	if c.Auth.Username != "" || c.Auth.Password != "" {
		return fmt.Errorf("auth.username/password Basic authentication is no longer supported; configure GitHub OAuth for users and auth.ci.api_keys for services")
	}
	if c.MaxBatchPackages < 0 || c.MaxBatchBytes < 0 {
		return fmt.Errorf("max_batch_packages and max_batch_bytes must not be negative")
	}
	if c.MaxBatchBytes > 0 && c.MaxBatchBytes < limits.PackageBytes(c.MaxSize)+limits.MaxSignatureBytes {
		return fmt.Errorf("max_batch_bytes must fit one max_size package plus a detached signature")
	}
	if len(c.Auth.SessionSecret) > 0 && c.Store.DBType == "cfkv" {
		return fmt.Errorf("auth.session_secret requires an atomic refresh/replay store; store.db_type cfkv cannot atomically consume refresh tokens (use sql or badgerdb)")
	}
	if c.Auth.AllowLegacySignerBasic && len(c.Auth.SessionSecret) == 0 {
		return fmt.Errorf("auth.allow_legacy_signer_basic requires auth.session_secret to verify the legacy CLI token")
	}
	if c.Miko.URL == "" && c.Miko.APIKey != "" {
		return fmt.Errorf("miko.api_key requires miko.url")
	}
	if c.Miko.URL != "" {
		if _, err := client.ParseBaseURL(c.Miko.URL); err != nil {
			return fmt.Errorf("miko.url: %w", err)
		}
		if c.Miko.APIKey == "" {
			return fmt.Errorf("miko.api_key is required when miko.url is configured")
		}
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
	if err := validateHTTPOrigin("auth.public_origin", c.Auth.PublicOrigin); err != nil {
		return err
	}
	if c.Auth.SelfOrigin != "" {
		if err := validateHTTPOrigin("auth.self_origin", c.Auth.SelfOrigin); err != nil {
			return err
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

// RepositoryCatalog maps the persistence schema into the validated repository
// domain. Runtime code consumes the catalog rather than reinterpreting config
// fields independently.
func (c *AyatoConfig) RepositoryCatalog() (*domain.RepositoryCatalog, error) {
	specs := make([]domain.RepositorySpec, 0, len(c.Repos))
	for _, repo := range c.Repos {
		specs = append(specs, domain.RepositorySpec{
			Name:                  repo.Name,
			Arches:                repo.Arches,
			AllowNewArch:          repo.AllowNewArch,
			TrustedKeys:           repo.TrustedKeys,
			Tiered:                repo.Tiered,
			PromotionKeepInSource: repo.PromotionKeepInSource,
			Upstream: domain.UpstreamSpec{
				DBURL:    repo.Upstream.DBURL,
				FilesURL: repo.Upstream.FilesURL,
			},
		})
	}
	return domain.NewRepositoryCatalog(c.DefaultArches, specs)
}
