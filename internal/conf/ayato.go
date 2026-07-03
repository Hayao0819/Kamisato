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

	"github.com/Hayao0819/Kamisato/internal/confloader"
	"github.com/spf13/pflag"
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
	ProtectedNames []string        `koanf:"protected_names,omitempty"`
	Port           int             `koanf:"port"`
	MaxSize        int             `koanf:"max_size"`
	Repos          []BinRepoConfig `koanf:"repos"`
	Auth           AuthConfig      `koanf:"auth"`
	Store          StoreConfig     `koanf:"store"`
	Build          BuildConfig     `koanf:"build"`
	Miko           MikoUpstream    `koanf:"miko"`
	Verify         VerifyConfig    `koanf:"verify"`
	AUR            AURConfig       `koanf:"aur"`
	BugReport      BugReportConfig `koanf:"bug_report"`
	Recaptcha      RecaptchaConfig `koanf:"recaptcha"`
	Sign           SignConfig      `koanf:"sign"`
	Secrets        SecretsConfig   `koanf:"secrets"`
	Pool           PoolConfig      `koanf:"pool"`
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
// /repo/<repo>/mirrorlist. Phase 1 advertises only this instance; peers and
// health filtering are future work.
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

// PoolConfig governs the content-addressed package pool: uploaded package bytes
// are stored once under pool/<sha256> and the repo path becomes a pointer, so
// identical content is deduplicated and old versions can be retained/collected.
type PoolConfig struct {
	// Enabled routes new package uploads through the pool. A pointer is nil-by-
	// default true, so pooling is on unless explicitly disabled; already-published
	// packages stored directly keep serving regardless.
	Enabled *bool `koanf:"enabled"`
	// KeepUnreferenced retains this many newest versions per pkgbase even once no
	// repo references them, so a rollback can re-point at a recent build.
	KeepUnreferenced int `koanf:"keep_unreferenced,omitempty"`
	// RetentionWindowHours keeps an unreferenced object at least this long after
	// its last pointer dropped, a grace window before the GC may reclaim it.
	RetentionWindowHours int `koanf:"retention_window_hours,omitempty"`
}

// PoolEnabled reports whether new uploads are pooled (default true).
func (c *AyatoConfig) PoolEnabled() bool {
	return c.Pool.Enabled == nil || *c.Pool.Enabled
}

// RetentionWindow is the GC grace window derived from the configured hours.
func (c PoolConfig) RetentionWindow() time.Duration {
	return time.Duration(c.RetentionWindowHours) * time.Hour
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

// RedirectDownloadsEnabled reports whether downloads should be redirected to a
// presigned URL. It defaults to true so egress offload is automatic; the redirect
// still only happens when the backend can actually presign.
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

type BuildConfig struct {
	Image     string `koanf:"image"`      // Docker image (default: "archlinux:latest")
	Timeout   int    `koanf:"timeout"`    // Build timeout in minutes (default: 30)
	GnupgHome string `koanf:"gnupg_home"` // GPG home directory for signing
	// ExtraRepos are pacman repositories added to the build environment (e.g. the
	// ayato repo) so already-published dependencies resolve during a build.
	ExtraRepos []ExtraRepo `koanf:"extra_repos"`
	// ResolveAURDeps, when set, makes miko resolve a target's unbuilt AUR
	// dependencies (from its .SRCINFO), build each in dependency order and publish
	// it to ayato before building the target. Opt-in; the target repo is exposed to
	// the build automatically so the freshly published dependencies resolve.
	ResolveAURDeps bool `koanf:"resolve_aur_deps"`
	// AURRPCURL overrides the aurweb RPC endpoint used to resolve AUR dependencies.
	// Empty uses the canonical AUR.
	AURRPCURL string `koanf:"aur_rpc_url"`
}

// ExtraRepo is a pacman repository exposed inside the build environment.
type ExtraRepo struct {
	Name     string `koanf:"name"`     // pacman repo name, e.g. "ayato"
	Server   string `koanf:"server"`   // Server line, e.g. https://repo.example.com/$repo/$arch
	SigLevel string `koanf:"siglevel"` // optional SigLevel; empty defaults to "Optional TrustAll"
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
	Name   string   `koanf:"name"`
	Arches []string `koanf:"arches"`
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

// Enabled reports whether the repo layers an upstream database.
func (u UpstreamRepoConfig) Enabled() bool { return u.DBURL != "" }

// DBURLFor resolves the upstream .db URL for one architecture.
func (u UpstreamRepoConfig) DBURLFor(arch string) string {
	return strings.ReplaceAll(u.DBURL, "$arch", arch)
}

// FilesURLFor resolves the upstream .files URL for one architecture, deriving it
// from the .db URL when not set explicitly.
func (u UpstreamRepoConfig) FilesURLFor(arch string) string {
	f := u.FilesURL
	if f == "" {
		f = deriveFilesURL(u.DBURL)
	}
	return strings.ReplaceAll(f, "$arch", arch)
}

// deriveFilesURL turns a ".db" URL into its ".files" sibling (extra.db ->
// extra.files, extra.db.tar.gz -> extra.files.tar.gz).
func deriveFilesURL(dbURL string) string {
	i := strings.LastIndex(dbURL, ".db")
	if i < 0 {
		return dbURL
	}
	return dbURL[:i] + ".files" + dbURL[i+len(".db"):]
}

// Tier names a stage in a tiered repo's staging -> testing -> stable flow.
type Tier string

const (
	TierStaging Tier = "staging"
	TierTesting Tier = "testing"
	TierStable  Tier = "stable"
)

// IsTierPromotion reports whether from -> to is a legal single promotion step
// (staging->testing or testing->stable). A package advances one tier at a time.
func IsTierPromotion(from, to Tier) bool {
	return (from == TierStaging && to == TierTesting) || (from == TierTesting && to == TierStable)
}

// TierRepo returns the physical pacman repo name serving a tier: stable is the
// bare name (so a client points at <name> as usual), staging and testing get the
// matching suffix, mirroring Arch's own core-staging/core-testing naming.
func (c *BinRepoConfig) TierRepo(t Tier) string {
	if t == TierStable {
		return c.Name
	}
	return c.Name + "-" + string(t)
}

// PhysicalRepos returns the pacman repo names this logical repo actually serves:
// its three tiers when tiered, otherwise just the bare name.
func (c *BinRepoConfig) PhysicalRepos() []string {
	if !c.Tiered {
		return []string{c.Name}
	}
	return []string{c.TierRepo(TierStaging), c.TierRepo(TierTesting), c.TierRepo(TierStable)}
}

func LoadAyatoConfig(flags *pflag.FlagSet, configFile string) (*AyatoConfig, error) {
	loadDotEnv()
	return confloader.LoadTyped[AyatoConfig](
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

// Validate rejects an OAuth config whose redirect_uri or session-cookie Secure flag
// could be derived from spoofable request headers: with GitHub login on,
// PublicOrigin is mandatory and must be an absolute http(s) origin without a path.
func (c *AyatoConfig) Validate() error {
	if err := c.Store.Validate(); err != nil {
		return err
	}
	if err := c.Store.checkStateless(UnderCloudRun()); err != nil {
		return err
	}
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

// PhysicalRepoNames is the servable pacman repo set: every configured repo
// expanded into the repos it actually serves (a tiered repo becomes its three
// tiers). Initialization and repo-name validation run over this set.
func (c *AyatoConfig) PhysicalRepoNames() []string {
	var out []string
	for i := range c.Repos {
		out = append(out, c.Repos[i].PhysicalRepos()...)
	}
	return out
}

func (c *AyatoConfig) Repo(name string) *BinRepoConfig {
	for _, repo := range c.Repos {
		if repo.Name == name {
			return &repo
		}
	}
	return nil
}

// UpstreamRepoNames lists the physical repo names that layer an upstream
// database, so the repository layer can serve their merged view.
func (c *AyatoConfig) UpstreamRepoNames() []string {
	var out []string
	for i := range c.Repos {
		if c.Repos[i].Upstream.Enabled() {
			out = append(out, c.Repos[i].PhysicalRepos()...)
		}
	}
	return out
}

// ResolveRepo returns the logical BinRepoConfig backing a physical pacman repo
// name — a tier of a tiered repo maps to its logical config — or nil if unknown.
func (c *AyatoConfig) ResolveRepo(physical string) *BinRepoConfig {
	for i := range c.Repos {
		for _, p := range c.Repos[i].PhysicalRepos() {
			if p == physical {
				return &c.Repos[i]
			}
		}
	}
	return nil
}
