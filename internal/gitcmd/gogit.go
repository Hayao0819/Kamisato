package gitcmd

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/transport"
)

// isPublic reports whether ip is a routable public address. It rejects exactly
// the address classes rejectInternalHost does, so the dialer can never reach
// loopback, private, link-local, or unspecified ranges (SSRF / cloud metadata).
func isPublic(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified())
}

// pinPublicDial resolves the host itself, keeps only public IPs, and dials one
// by its literal address. Pinning to an IP validated in the same step closes the
// DNS-rebinding TOCTOU. It backs the strict transport options only.
func pinPublicDial(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to resolve remote host")
	}
	var lastErr error
	for _, ip := range ips {
		if !isPublic(ip) {
			lastErr = errors.NewErrf("remote host %s resolves to a non-public address", host)
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
		lastErr = errors.NewErrf("remote host %s did not resolve to any address", host)
	}
	return nil, lastErr
}

// strictClientOptions returns the go-git transport options for a strict
// (untrusted) clone: an https client and an ssh/git dialer that both pin to a
// validated public IP, and an https client that refuses redirects — closing the
// SSRF / DNS-rebinding window on every transport. go-git v6 takes these
// per-operation, so non-strict operations keep the default client and can reach
// loopback/private git servers.
func strictClientOptions() []client.Option {
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext:           pinPublicDial,
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
	return []client.Option{
		client.WithHTTPClient(httpClient),
		client.WithDialer(transport.DialContextFunc(pinPublicDial)),
	}
}

// cloneGoGit clones through go-git. A strict clone passes the pinning client
// options (see strictClientOptions); ValidateRemote also runs first (in Clone)
// as defense in depth.
func cloneGoGit(ctx context.Context, o CloneOptions) error {
	opts := &git.CloneOptions{URL: o.URL, Depth: o.Depth, Bare: o.Bare}
	if o.Strict {
		opts.ClientOptions = strictClientOptions()
	}
	repo, err := git.PlainCloneContext(ctx, o.Dir, opts)
	if err != nil {
		return errors.WrapErr(err, "git clone: "+o.URL)
	}
	if o.Ref == "" || o.Bare {
		return nil
	}
	wt, err := repo.Worktree()
	if err != nil {
		return errors.WrapErr(err, "open worktree")
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(o.Ref))
	if err != nil {
		return errors.WrapErr(err, "resolve ref "+o.Ref)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
		return errors.WrapErr(err, "checkout "+o.Ref)
	}
	return nil
}

// Pull fast-forwards the checkout in dir from origin via go-git (no git process
// spawned). It returns nil when already up to date and errors on divergence (like
// the CLI's --ff-only). It is non-strict, so loopback/private origins stay reachable.
func Pull(ctx context.Context, dir string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return errors.WrapErr(err, "open repo "+dir)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return errors.WrapErr(err, "open worktree")
	}
	if err := wt.PullContext(ctx, &git.PullOptions{RemoteName: "origin"}); err != nil &&
		!errors.Is(err, git.NoErrAlreadyUpToDate) {
		return errors.WrapErr(err, "git pull")
	}
	return nil
}

// SyncHard force-syncs the checkout in dir to ref — a branch, tag, or commit on
// origin — by fetching all branches and tags with prune and hard-resetting the
// working tree to it. It is the go-git equivalent of
// `git fetch --tags --prune origin <ref>` + `git reset --hard FETCH_HEAD`, and is
// non-strict (overlay origins may be loopback/private).
func SyncHard(ctx context.Context, dir, ref string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return errors.WrapErr(err, "open repo "+dir)
	}
	err = repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{"+refs/heads/*:refs/remotes/origin/*"},
		Tags:       git.AllTags,
		Prune:      true,
		Force:      true,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return errors.WrapErr(err, "git fetch")
	}
	// ref may name a remote branch, a tag, or a commit — resolve in that order.
	hash, err := repo.ResolveRevision(plumbing.Revision("origin/" + ref))
	if err != nil {
		hash, err = repo.ResolveRevision(plumbing.Revision(ref))
		if err != nil {
			return errors.WrapErr(err, "resolve ref "+ref)
		}
	}
	wt, err := repo.Worktree()
	if err != nil {
		return errors.WrapErr(err, "open worktree")
	}
	if err := wt.Reset(&git.ResetOptions{Mode: git.HardReset, Commit: *hash}); err != nil {
		return errors.WrapErr(err, "git reset --hard")
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
