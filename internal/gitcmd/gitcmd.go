// Package gitcmd is the project's git access layer, implemented entirely with
// go-git — no git process is ever spawned, including for submodules. Strict
// clones validate the remote URL and use a transport client that pins every
// connection to a validated public IP and refuses redirects, closing the SSRF /
// DNS-rebinding window required when the URL comes from an untrusted caller (e.g.
// ayato's register endpoint, an SSRF into cloud metadata otherwise); non-strict
// operations use the default client so operator/loopback git servers stay
// reachable. go-git also has no ext:: transport, so that RCE surface is absent.
package gitcmd

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

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

// Clone clones o.URL into o.Dir via go-git (no git process spawned). A strict
// clone validates the URL and dials with the public-IP pin + redirect refusal
// (see safeDialContext); a non-strict clone dials normally so operator/local
// origins stay reachable. go-git has no ext:: transport, so the CLI's ext:: RCE
// surface is absent here by construction.
func Clone(ctx context.Context, o CloneOptions) error {
	if o.Strict {
		if err := ValidateRemote(o.URL); err != nil {
			return err
		}
	}
	return cloneGoGit(ctx, o)
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

// rejectInternalHost rejects a host resolving to an internal address. It is
// defense in depth: for strict operations the go-git transport dialer (see
// gogit.go) independently re-resolves and pins the connection to a validated
// public IP on every scheme (https/ssh/git), closing the DNS-rebinding TOCTOU
// this early check alone would leave open.
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
