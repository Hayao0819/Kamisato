package buildset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/filesystem/safefile"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	pacmanpkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/internal/vcs/git"
	"github.com/otiai10/copy"
)

type sourceRecord struct {
	metadata     *pacmanpkg.SourcePackage
	ref          Source
	workDir      string
	explicit     map[string]struct{}
	dependencies []Dependency
}

func defaultCloneSource(ctx context.Context, sourceURL, dir string) (string, error) {
	if err := git.Clone(ctx, git.CloneOptions{URL: sourceURL, Dir: dir, Depth: 1, Strict: true}); err != nil {
		return "", err
	}
	return git.HeadCommit(ctx, dir)
}

func (s *Service) collectLocalSources(
	ctx context.Context,
	sources []string,
	sourcesDir, arch, pacmanConf string,
	localRepositoryDirs []string,
) ([]*sourceRecord, error) {
	var records []*sourceRecord
	pkgbases := map[string]string{}
	for index, source := range sources {
		input, err := filepath.Abs(source)
		if err != nil {
			return nil, fmt.Errorf("resolve local source %q: %w", source, err)
		}
		info, err := os.Stat(input)
		if err != nil {
			return nil, fmt.Errorf("inspect local source %q: %w", input, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("local source %q is not a directory", input)
		}
		if _, err := os.Stat(filepath.Join(input, "PKGBUILD")); err != nil {
			return nil, fmt.Errorf("local source %q has no PKGBUILD: %w", input, err)
		}
		resolved, err := filepath.EvalSymlinks(input)
		if err != nil {
			return nil, fmt.Errorf("resolve local source %q: %w", input, err)
		}
		destination := filepath.Join(sourcesDir, "local", fmt.Sprintf("%04d", index))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return nil, err
		}
		if err := copy.Copy(resolved, destination); err != nil {
			return nil, fmt.Errorf("copy local source %q: %w", input, err)
		}
		metadata, err := s.openMetadata(ctx, destination, arch, pacmanConf, localRepositoryDirs)
		if err != nil {
			return nil, fmt.Errorf("read local source %q: %w", input, err)
		}
		if err := validateSourceMetadata(metadata, arch); err != nil {
			return nil, fmt.Errorf("validate local source %q: %w", input, err)
		}
		if previous, exists := pkgbases[metadata.Base()]; exists {
			return nil, fmt.Errorf("pkgbase %q is defined by both %s and %s", metadata.Base(), previous, input)
		}
		outputs := metadata.OutputNames(arch)
		pkgbases[metadata.Base()] = input
		digest, err := digestTree(destination)
		if err != nil {
			return nil, fmt.Errorf("digest local source %q: %w", input, err)
		}
		explicit := make(map[string]struct{}, len(outputs))
		for _, name := range outputs {
			explicit[name] = struct{}{}
		}
		records = append(records, &sourceRecord{
			metadata: metadata,
			ref:      Source{Type: SourceLocal, Path: input, Digest: digest},
			workDir:  destination,
			explicit: explicit,
		})
	}
	slices.SortFunc(records, func(left, right *sourceRecord) int {
		return strings.Compare(left.metadata.Base(), right.metadata.Base())
	})
	return records, nil
}

func (s *Service) fetchAURSource(
	ctx context.Context,
	pkgbase, sourcesDir, arch, pacmanConf string,
	localRepositoryDirs []string,
) (*sourceRecord, error) {
	if err := validatePackageName(pkgbase); err != nil {
		return nil, err
	}
	url := strings.TrimRight(s.aur.GitBase(), "/") + "/" + pkgbase + ".git"
	destination := filepath.Join(sourcesDir, "aur", pkgbase)
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return nil, err
	}
	revision, err := s.clone(ctx, url, destination)
	if err != nil {
		return nil, fmt.Errorf("clone AUR pkgbase %q: %w", pkgbase, err)
	}
	metadata, err := s.openMetadata(ctx, destination, arch, pacmanConf, localRepositoryDirs)
	if err != nil {
		return nil, fmt.Errorf("read AUR pkgbase %q: %w", pkgbase, err)
	}
	if err := validateSourceMetadata(metadata, arch); err != nil {
		return nil, fmt.Errorf("validate AUR pkgbase %q: %w", pkgbase, err)
	}
	if metadata.Base() != pkgbase {
		return nil, fmt.Errorf("AUR source %q declares pkgbase %q", pkgbase, metadata.Base())
	}
	digest, err := digestTree(destination)
	if err != nil {
		return nil, fmt.Errorf("digest AUR source %q: %w", pkgbase, err)
	}
	return &sourceRecord{
		metadata: metadata,
		ref:      Source{Type: SourceAUR, URL: url, Revision: revision, Digest: digest},
		workDir:  destination,
		explicit: map[string]struct{}{},
	}, nil
}

func validateSourceMetadata(metadata *pacmanpkg.SourcePackage, arch string) error {
	for _, name := range metadata.Names() {
		if err := validatePackageName(name); err != nil {
			return err
		}
	}
	if len(metadata.OutputNames(arch)) == 0 {
		return fmt.Errorf("pkgbase %q does not support architecture %s", metadata.Base(), arch)
	}
	return nil
}

func (s *Service) openMetadata(
	ctx context.Context,
	dir, arch, pacmanConf string,
	localRepositoryDirs []string,
) (*pacmanpkg.SourcePackage, error) {
	metadata, err := pacmanpkg.OpenSourcePackage(dir)
	if err == nil {
		return metadata, nil
	}
	if err != pacmanpkg.ErrSRCINFONotFound {
		return nil, err
	}
	if s.metadata == nil {
		return nil, fmt.Errorf(".SRCINFO is missing and no isolated metadata backend is configured")
	}
	data, err := s.metadata.GenerateSRCINFO(ctx, builder.Spec{
		SrcDir: dir, Arch: arch, PacmanConf: pacmanConf, LocalRepositoryDirs: localRepositoryDirs,
	})
	if err != nil {
		return nil, fmt.Errorf("generate .SRCINFO in isolation: %w", err)
	}
	if err := safefile.WriteFile(filepath.Join(dir, ".SRCINFO"), data, 0o644); err != nil {
		return nil, fmt.Errorf("write generated .SRCINFO: %w", err)
	}
	return pacmanpkg.OpenSourcePackage(dir)
}

func digestTree(root string) (string, error) {
	hash := sha256.New()
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if relative == "." {
			return nil
		}
		if _, err := io.WriteString(hash, filepath.ToSlash(relative)+"\x00"); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(hash, "%o\x00", info.Mode()); err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := rootFS.Readlink(relative)
			if err != nil {
				return err
			}
			_, err = io.WriteString(hash, target+"\x00")
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := rootFS.Open(relative)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	closeErr := rootFS.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
