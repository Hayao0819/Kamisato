// Package artifact finds and moves package files produced by build backends.
package artifact

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/pkg/pacman/pkgfile"
	"github.com/otiai10/copy"
)

type fileState struct {
	info os.FileInfo
}

// Baseline distinguishes same-version rebuilds from stale package files.
type Baseline map[string]fileState

func Snapshot(dir string) (Baseline, error) {
	set := Baseline{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return set, nil
		}
		return nil, fmt.Errorf("failed to snapshot package dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && IsPackageFile(entry.Name()) {
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("failed to stat existing package %s: %w", entry.Name(), err)
			}
			set[entry.Name()] = fileState{info: info}
		}
	}
	return set, nil
}

// Collect ignores package files unchanged from baseline.
func Collect(dir string, baseline Baseline) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read package dir: %w", err)
	}
	var packages []string
	for _, entry := range entries {
		if entry.IsDir() || !IsPackageFile(entry.Name()) {
			continue
		}
		if previous, ok := baseline[entry.Name()]; ok {
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("failed to stat package %s: %w", entry.Name(), err)
			}
			if os.SameFile(previous.info, info) &&
				previous.info.Size() == info.Size() &&
				previous.info.ModTime().Equal(info.ModTime()) {
				continue
			}
		}
		abs, err := filepath.Abs(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve package path: %w", err)
		}
		packages = append(packages, abs)
	}
	return packages, nil
}

func MoveToDir(built []string, srcDir, outDir string) ([]string, error) {
	if outDir == "" {
		outDir = srcDir
	}
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve src dir: %w", err)
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve out dir: %w", err)
	}
	if absOut == absSrc {
		return built, nil
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil { //nolint:gosec // build output is intentionally world-readable
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	packages := make([]string, 0, len(built))
	for _, path := range built {
		dst := filepath.Join(absOut, filepath.Base(path))
		if err := moveFile(path, dst); err != nil {
			return nil, fmt.Errorf("failed to move package to output directory: %w", err)
		}
		packages = append(packages, dst)
	}
	return packages, nil
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copy.Copy(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func IsPackageFile(name string) bool {
	return pkgfile.IsArchive(name)
}
