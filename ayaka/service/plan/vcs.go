package plan

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	pkg "github.com/Hayao0819/Kamisato/internal/pacman"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/internal/pacman/source"
)

const gitRemoteTimeout = 15 * time.Second

var (
	describedGitCommit = regexp.MustCompile(`(?i)\.g([0-9a-f]{7,})`)
	revisionGitCommit  = regexp.MustCompile(`(?i)(?:^|\.)r[0-9]+\.([0-9a-f]{7,})(?:$|[-+.])`)
)

type gitCommitResolver func(context.Context, source.GitSource) (string, error)

type vcsResult struct {
	pkgbase string
	changed bool
	errs    []error
}

func detectVCSUpdates(
	ctx context.Context,
	packages []*pkg.SourcePackage,
	remote *repo.RemoteRepo,
	arch string,
	resolve gitCommitResolver,
) map[string]struct{} {
	results := make(chan vcsResult, len(packages))
	var checks sync.WaitGroup

	for _, sourcePackage := range packages {
		published := remote.PkgByPkgBase(sourcePackage.Base())
		if published == nil {
			continue
		}

		gitSources := make([]source.GitSource, 0)
		for _, value := range sourcePackage.Sources(arch) {
			if parsed, ok := source.ParseGitSource(value); ok {
				gitSources = append(gitSources, parsed)
			}
		}
		if len(gitSources) == 0 {
			continue
		}

		checks.Add(1)
		go func() {
			defer checks.Done()
			changed, errs := gitSourcesChanged(ctx, published.Version(), gitSources, resolve)
			results <- vcsResult{pkgbase: sourcePackage.Base(), changed: changed, errs: errs}
		}()
	}

	go func() {
		checks.Wait()
		close(results)
	}()

	updates := make(map[string]struct{})
	for result := range results {
		for _, err := range result.errs {
			slog.Warn("failed to check VCS source", "pkgbase", result.pkgbase, "error", err)
		}
		if result.changed {
			updates[result.pkgbase] = struct{}{}
		}
	}
	return updates
}

func gitSourcesChanged(
	ctx context.Context,
	publishedVersion string,
	sources []source.GitSource,
	resolve gitCommitResolver,
) (bool, []error) {
	publishedCommit, ok := gitCommitFromVersion(publishedVersion)
	if !ok {
		return true, nil
	}

	checkCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		commit string
		err    error
	}
	results := make(chan result, len(sources))
	for _, gitSource := range sources {
		go func() {
			commit, err := resolve(checkCtx, gitSource)
			results <- result{commit: commit, err: err}
		}()
	}

	var errs []error
	for range sources {
		result := <-results
		if result.err != nil {
			if ctx.Err() == nil {
				errs = append(errs, result.err)
			}
			continue
		}
		if !strings.HasPrefix(strings.ToLower(result.commit), publishedCommit) {
			return true, errs
		}
	}
	return false, errs
}

func gitRemoteCommit(ctx context.Context, gitSource source.GitSource) (string, error) {
	lookupCtx, cancel := context.WithTimeout(ctx, gitRemoteTimeout)
	defer cancel()

	cmd := exec.CommandContext(lookupCtx, "git", "ls-remote", "--", gitSource.Remote, gitSource.Ref) //nolint:gosec // the maintainer's PKGBUILD supplies the remote, and argv does not pass through a shell
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if lookupCtx.Err() != nil {
			return "", lookupCtx.Err()
		}
		detail := strings.ReplaceAll(strings.TrimSpace(stderr.String()), gitSource.Remote, "<remote>")
		if detail == "" {
			return "", fmt.Errorf("git ls-remote failed: %w", err)
		}
		return "", fmt.Errorf("git ls-remote failed: %w: %s", err, detail)
	}

	wantRef := gitSource.Ref
	if wantRef != "HEAD" && !strings.HasPrefix(wantRef, "refs/") {
		wantRef = "refs/heads/" + wantRef
	}
	for line := range strings.Lines(stdout.String()) {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != wantRef || !validGitCommit(fields[0]) {
			continue
		}
		return strings.ToLower(fields[0]), nil
	}
	return "", fmt.Errorf("git ls-remote returned no commit for %s", gitSource.Ref)
}

func gitCommitFromVersion(version string) (string, bool) {
	matches := describedGitCommit.FindAllStringSubmatch(version, -1)
	if len(matches) == 0 {
		matches = revisionGitCommit.FindAllStringSubmatch(version, -1)
	}
	if len(matches) == 0 {
		return "", false
	}
	commit := strings.ToLower(matches[len(matches)-1][1])
	if !validGitCommitPrefix(commit) {
		return "", false
	}
	return commit, true
}

func validGitCommit(commit string) bool {
	return (len(commit) == 40 || len(commit) == 64) && validHex(commit)
}

func validGitCommitPrefix(commit string) bool {
	return len(commit) >= 7 && len(commit) <= 64 && validHex(commit)
}

func validHex(value string) bool {
	if len(value)%2 != 0 {
		value = "0" + value
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
