package nvcheck

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	alpm "github.com/Hayao0819/dyalpm"
)

// gitTagSource lists a remote's tags with git ls-remote; --refs drops the
// peeled ^{} duplicates of annotated tags.
type gitTagSource struct {
	url    string
	prefix string
}

func (s *gitTagSource) Latest(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--tags", "--refs", s.url) //nolint:gosec // url comes from the maintainer's .nvchecker.toml, like the PKGBUILD next to it
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("nvcheck: git ls-remote %s: %w: %s", s.url, err, strings.TrimSpace(errBuf.String()))
	}
	best := ""
	for line := range strings.Lines(out.String()) {
		_, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		tag, ok := strings.CutPrefix(ref, "refs/tags/")
		if !ok {
			continue
		}
		v := stripPrefix(tag, s.prefix)
		if v == "" {
			continue
		}
		if best == "" || alpm.VerCmp(v, best) > 0 {
			best = v
		}
	}
	if best == "" {
		return "", fmt.Errorf("nvcheck: no tags found at %s", s.url)
	}
	return best, nil
}
