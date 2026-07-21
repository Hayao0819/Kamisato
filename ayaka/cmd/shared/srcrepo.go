package shared

import (
	"io"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// ResolveDiffServer picks the remote repo db dir: the explicit --diff-url, else
// the deprecated --server, else the arch-less repo.json url with the arch
// appended. Empty when none is configured.
func ResolveDiffServer(diffURL, server, configURL, arch string) string {
	if diffURL != "" {
		return diffURL
	}
	if server != "" {
		return server
	}
	if configURL != "" {
		return strings.TrimRight(configURL, "/") + "/" + arch
	}
	return ""
}

// ReloadWithSrcinfo regenerates every .SRCINFO in srcrepo and reloads it so the
// fresh versions drive the diff or plan; returned unchanged when makepkg is
// absent (e.g. CI without pacman tooling).
func ReloadWithSrcinfo(srcrepo *repo.SourceRepo, stderr io.Writer) (*repo.SourceRepo, error) {
	if _, err := exec.LookPath("makepkg"); err != nil {
		slog.Warn("skipping .SRCINFO update: makepkg not found on PATH", "error", err)
		return srcrepo, nil
	}
	srcdirs, err := repo.GetSrcDirs(srcrepo.Dir)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to list source directories")
	}
	for _, d := range srcdirs {
		if err := repo.GenerateSrcinfo(d, stderr); err != nil {
			slog.Warn("failed to update .SRCINFO", "dir", d, "error", err)
		}
	}
	reloaded, err := repo.GetSrcRepo(srcrepo.Dir, srcrepo.Config)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to reload source repo after .SRCINFO update")
	}
	reloaded.Dir = srcrepo.Dir
	reloaded.DestDir = srcrepo.DestDir
	return reloaded, nil
}
