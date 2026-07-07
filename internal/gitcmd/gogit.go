package gitcmd

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errwrap"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// isHTTPSURL reports whether raw is an https:// URL. Strict https clones take
// the go-git path; other strict schemes (ssh/git) stay on the hardened CLI.
func isHTTPSURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https"
}

// isPublic reports whether ip is a routable public address. It rejects exactly
// the address classes rejectInternalHost does, so the dialer can never reach
// loopback, private, link-local, or unspecified ranges (SSRF / cloud metadata).
func isPublic(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified())
}

// safeDialContext resolves the host itself, keeps only public IPs, and dials
// one of them by its literal address. Pinning the connection to an IP that was
// validated in the same step closes the DNS-rebinding TOCTOU that the git CLI
// leaves open, since nothing re-resolves the name between check and connect.
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, errwrap.WrapErr(err, "failed to resolve remote host")
	}
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range ips {
		if !isPublic(ip) {
			lastErr = errwrap.NewErrf("remote host %s resolves to a non-public address", host)
			continue
		}
		conn, dErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dErr != nil {
			lastErr = dErr
			continue
		}
		return conn, nil
	}
	if lastErr == nil {
		lastErr = errwrap.NewErrf("remote host %s did not resolve to any address", host)
	}
	return nil, lastErr
}

var installSafeClientOnce sync.Once

// installSafeHTTPSClient registers, once, an https transport for go-git that
// pins connections to validated public IPs and refuses redirects (a redirect to
// an internal host would otherwise bypass the resolved-IP check).
func installSafeHTTPSClient() {
	installSafeClientOnce.Do(func() {
		safeClient := &http.Client{
			Transport: &http.Transport{
				DialContext:           safeDialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		client.InstallProtocol("https", githttp.NewClient(safeClient))
	})
}

// cloneGoGit performs a strict https clone through go-git so the connection is
// pinned to a validated public IP. ValidateRemote still runs first (in Clone)
// as defense in depth.
func cloneGoGit(ctx context.Context, o CloneOptions) error {
	installSafeHTTPSClient()

	// Clone the default branch (with its tags) at the requested depth, then check
	// out Ref by resolving it — a branch, tag, or commit — so this matches the
	// git CLI's `clone --depth N` + `checkout <ref>` semantics rather than
	// accepting only branch names. An off-history ref has the same shallow-clone
	// limitation the CLI path had.
	repo, err := git.PlainCloneContext(ctx, o.Dir, o.Bare, &git.CloneOptions{URL: o.URL, Depth: o.Depth})
	if err != nil {
		return errwrap.WrapErr(err, "git clone: "+o.URL)
	}
	if o.Ref == "" || o.Bare {
		return nil
	}
	wt, err := repo.Worktree()
	if err != nil {
		return errwrap.WrapErr(err, "open worktree")
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(o.Ref))
	if err != nil {
		return errwrap.WrapErr(err, "resolve ref "+o.Ref)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
		return errwrap.WrapErr(err, "checkout "+o.Ref)
	}
	return nil
}

// Pull fast-forwards the checkout in dir from its origin, through go-git so no
// git process is spawned. It returns nil when already up to date, and errors
// (like the CLI's --ff-only) when the local branch has diverged. https fetches
// are pinned to a validated public IP, the same as clones.
func Pull(ctx context.Context, dir string) error {
	installSafeHTTPSClient()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return errwrap.WrapErr(err, "open repo "+dir)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return errwrap.WrapErr(err, "open worktree")
	}
	if err := wt.PullContext(ctx, &git.PullOptions{RemoteName: "origin"}); err != nil &&
		!errors.Is(err, git.NoErrAlreadyUpToDate) {
		return errwrap.WrapErr(err, "git pull")
	}
	return nil
}

// CommitUnix returns the HEAD commit time as a unix timestamp, or 0 on failure.
func CommitUnix(_ context.Context, dir string) int64 {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return 0
	}
	head, err := repo.Head()
	if err != nil {
		return 0
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return 0
	}
	return commit.Committer.When.Unix()
}
