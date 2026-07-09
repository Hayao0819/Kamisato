// Package gitcmd is the one place the project shells out to git. Every
// invocation runs with hardened defaults — no terminal auth prompt and the
// ext:: transport disabled (ext:: runs an arbitrary command, i.e. RCE) — and
// arguments are always passed as a vector, never a shell string. Strict clones
// additionally validate the remote URL and refuse local-path/file transports
// and private-network hosts, which is required when the URL comes from an
// untrusted caller (e.g. ayato's register endpoint, where it would otherwise be
// an SSRF into cloud metadata). Strict https clones are served by go-git instead
// of the CLI (see gogit.go) so the connection can be pinned to a validated
// public IP, which the git CLI cannot do.
package gitcmd

import (
	"context"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

// hardenedConfig is prepended to every git invocation.
var hardenedConfig = []string{"-c", "protocol.ext.allow=never"}

func command(ctx context.Context, dir string, args, extraConfig []string) *exec.Cmd {
	if ctx == nil {
		ctx = context.Background()
	}
	full := make([]string, 0, len(hardenedConfig)+len(extraConfig)+len(args))
	full = append(full, hardenedConfig...)
	full = append(full, extraConfig...)
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, "git", full...) //nolint:gosec // fixed program git, argv passed as separate args (no shell)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

// Run executes a git subcommand in dir (or the cwd when dir is "").
func Run(ctx context.Context, dir string, args ...string) error {
	out, err := command(ctx, dir, args, nil).CombinedOutput()
	if err != nil {
		return errors.WrapErr(err, "git "+strings.Join(args, " ")+": "+strings.TrimSpace(string(out)))
	}
	return nil
}

type CloneOptions struct {
	URL   string
	Dir   string
	Ref   string // checked out after clone when set
	Depth int    // shallow depth; 0 = full history
	// Strict validates URL against ValidateRemote and disables the local-file
	// transport. Use it whenever URL is not fully trusted.
	Strict bool
	// Bare clones without a working tree (a servable repository).
	Bare bool
}

func Clone(ctx context.Context, o CloneOptions) error {
	var extra []string
	if o.Strict {
		if err := ValidateRemote(o.URL); err != nil {
			return err
		}
		// Strict https clones go through go-git, whose dialer pins the
		// connection to a validated public IP and so closes the DNS-rebinding
		// TOCTOU the CLI leaves open (see rejectInternalHost). ssh/git strict
		// clones stay on the hardened CLI below.
		if isHTTPSURL(o.URL) {
			return cloneGoGit(ctx, o)
		}
		extra = append(extra, "-c", "protocol.file.allow=never")
		// The caller controls the registered server, so a 3xx redirect to an
		// internal host would bypass ValidateRemote, which only inspects the
		// initial URL.
		extra = append(extra, "-c", "http.followRedirects=false")
	}

	args := []string{"clone", "--quiet"}
	if o.Bare {
		args = append(args, "--bare")
	}
	if o.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(o.Depth))
	}
	args = append(args, "--", o.URL, o.Dir)

	if out, err := command(ctx, "", args, extra).CombinedOutput(); err != nil {
		return errors.WrapErr(err, "git clone: "+strings.TrimSpace(string(out)))
	}
	if o.Ref != "" && !o.Bare {
		return Run(ctx, o.Dir, "checkout", "--quiet", o.Ref)
	}
	return nil
}

// ValidateRemote rejects remotes an untrusted caller could abuse: only https,
// git, and ssh are allowed (plaintext http, file paths, and ext:: refused), and a
// host resolving to a private, loopback, or link-local address is rejected to
// narrow SSRF. It inspects only the initial URL and first resolution, so it does
// not fully close SSRF (see rejectInternalHost).
func ValidateRemote(raw string) error {
	// "<helper>::<addr>" is git's transport-helper syntax; ext:: runs an
	// arbitrary command. Reject it before any scp-like heuristic can match it.
	if strings.Contains(raw, "::") {
		return errors.NewErr("transport-helper (::) remotes are not allowed")
	}
	if host, ok := scpLikeSSH(raw); ok {
		return rejectInternalHost(host)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.WrapErr(err, "invalid remote URL")
	}

	switch u.Scheme {
	case "ssh", "https", "git":
		return rejectInternalHost(u.Hostname())
	case "http":
		return errors.NewErr("plaintext http remotes are not allowed")
	case "":
		return errors.NewErr("local-path remotes are not allowed for untrusted sources")
	default:
		return errors.NewErrf("remote scheme %q is not allowed", u.Scheme)
	}
}

// scpLikeSSH matches git's scp-style "user@host:path" form (no scheme, has a
// colon before any slash) and returns the host so the caller can SSRF-check it.
func scpLikeSSH(raw string) (host string, ok bool) {
	if strings.Contains(raw, "://") {
		return "", false
	}
	colon := strings.IndexByte(raw, ':')
	slash := strings.IndexByte(raw, '/')
	if colon <= 0 || (slash != -1 && colon >= slash) {
		return "", false
	}
	host = raw[:colon]
	if at := strings.LastIndexByte(host, '@'); at != -1 {
		host = host[at+1:]
	}
	return host, true
}

// rejectInternalHost rejects a host resolving to an internal address, but the
// check and the clone resolve the name separately, leaving a DNS-rebinding TOCTOU.
// For strict https it is closed downstream (Clone routes https through go-git,
// which pins the connection to the checked public IP). ssh and git:// stay on the
// CLI and re-resolve at connect time, so a DNS-controlling caller can rebind to an
// internal IP after this check — only network egress restriction fully closes that.
func rejectInternalHost(host string) error {
	if host == "" {
		return errors.NewErr("remote URL has no host")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return errors.WrapErr(err, "failed to resolve remote host")
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return errors.NewErrf("remote host %s resolves to a non-public address", host)
		}
	}
	return nil
}
