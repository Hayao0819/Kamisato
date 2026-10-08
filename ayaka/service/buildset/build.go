package buildset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/filesystem/safefile"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	pacmanpkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
)

type builtPackage struct {
	path      string
	signature string
	buildInfo string
	manifest  ManifestPackage
}

type builtSource struct {
	packages []builtPackage
	log      string
}

func (s *Service) Build(ctx context.Context, request BuildRequest) (_ *Manifest, returnErr error) {
	if len(request.Packages) == 0 && len(request.LocalSources) == 0 {
		return nil, nil
	}
	if s.backend == nil {
		return nil, fmt.Errorf("build backend is required")
	}
	if s.backend.Name() == "chroot" {
		return nil, fmt.Errorf("direct package builds do not use the chroot backend because makechrootpkg evaluates PKGBUILD on the host; use --backend container")
	}
	if err := validateBuildRequest(request); err != nil {
		return nil, err
	}

	output, err := filepath.Abs(request.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve repository output: %w", err)
	}
	manifestPath, err := filepath.Abs(request.Manifest)
	if err != nil {
		return nil, fmt.Errorf("resolve manifest path: %w", err)
	}
	if pathWithin(output, manifestPath) {
		return nil, fmt.Errorf("manifest path %q must be outside repository output %q", manifestPath, output)
	}
	if manifestPath == output+".lock" || output == manifestPath+".lock" {
		return nil, fmt.Errorf("repository output and manifest must not overlap each other's lock paths")
	}
	if err := validatePathOutside(output, request.WorkDir, "work directory"); err != nil {
		return nil, err
	}
	if err := validatePathOutside(output, request.LogDir, "log directory"); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return nil, fmt.Errorf("create repository output parent: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o750); err != nil {
		return nil, fmt.Errorf("create manifest parent: %w", err)
	}
	outputLock, err := safefile.TryLock(output+".lock", 0o600)
	if err != nil {
		return nil, fmt.Errorf("lock repository output: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, outputLock.Unlock())
	}()
	manifestLock, err := safefile.TryLock(manifestPath+".lock", 0o600)
	if err != nil {
		return nil, fmt.Errorf("lock manifest output: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, manifestLock.Unlock())
	}()
	if _, err := os.Lstat(output); err == nil {
		return nil, fmt.Errorf("repository output %q already exists", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect repository output %q: %w", output, err)
	}
	if _, err := os.Lstat(manifestPath); err == nil {
		return nil, fmt.Errorf("manifest path %q already exists", manifestPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect manifest path %q: %w", manifestPath, err)
	}

	request.OutputDir = output
	request.Manifest = manifestPath
	prepared, cleanup, err := s.prepare(ctx, request.PlanRequest)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if len(prepared.plan.Builds) == 0 {
		return nil, nil
	}

	logsRoot := request.LogDir
	externalLogs := logsRoot != ""
	if externalLogs {
		logsRoot, err = filepath.Abs(logsRoot)
		if err != nil {
			return nil, fmt.Errorf("resolve log directory: %w", err)
		}
	} else {
		logsRoot = filepath.Join(prepared.workspace, "logs")
	}
	if err := os.MkdirAll(logsRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	built := make(map[string]builtSource, len(prepared.plan.Builds))
	for _, planned := range prepared.plan.Builds {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		source := prepared.sources[planned.Pkgbase]
		if source == nil {
			return nil, fmt.Errorf("planned pkgbase %q has no prepared source", planned.Pkgbase)
		}
		outDir := filepath.Join(prepared.workspace, "artifacts", planned.Pkgbase)
		if err := os.MkdirAll(outDir, 0o700); err != nil {
			return nil, err
		}
		logPath := filepath.Join(logsRoot, planned.Pkgbase+".log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, fmt.Errorf("open build log for %q: %w", planned.Pkgbase, err)
		}
		installPackages, err := dependencyArtifacts(planned.Pkgbase, prepared.plan, built)
		if err != nil {
			_ = logFile.Close()
			return nil, err
		}
		result, buildErr := s.backend.Build(ctx, builder.Spec{
			SrcDir: source.workDir, OutDir: outDir, Arch: request.Arch,
			PacmanConf: prepared.pacmanConf, LocalRepositoryDirs: prepared.localRepositoryDirs,
			InstallPkgs: installPackages, LogWriter: logFile,
		})
		closeErr := logFile.Close()
		if buildErr != nil {
			return nil, errors.Join(fmt.Errorf("build pkgbase %q: %w", planned.Pkgbase, buildErr), closeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close build log for %q: %w", planned.Pkgbase, closeErr)
		}
		packages, err := validateBuildResult(result, source, request.Arch, filepath.Join(prepared.workspace, "buildinfo", planned.Pkgbase))
		if err != nil {
			return nil, fmt.Errorf("validate pkgbase %q output: %w", planned.Pkgbase, err)
		}
		built[planned.Pkgbase] = builtSource{packages: packages, log: logPath}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	manifest, err := s.publish(request, prepared.plan, built, output, manifestPath, externalLogs)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func validateBuildRequest(request BuildRequest) error {
	if err := repo.ValidateName(request.RepoName); err != nil {
		return err
	}
	if request.OutputDir == "" {
		return fmt.Errorf("repository output is required")
	}
	if request.Manifest == "" {
		return fmt.Errorf("manifest path is required")
	}
	return nil
}

func dependencyArtifacts(base string, plan *Plan, built map[string]builtSource) ([]string, error) {
	byBase := make(map[string]PlannedBuild, len(plan.Builds))
	for _, build := range plan.Builds {
		byBase[build.Pkgbase] = build
	}
	seen := map[string]struct{}{}
	var paths []string
	var visit func(string) error
	visit = func(current string) error {
		for _, dependency := range byBase[current].Dependencies {
			provider := dependency.ProviderPkgbase
			if provider == "" || provider == current {
				continue
			}
			if _, exists := seen[provider]; exists {
				continue
			}
			if _, exists := byBase[provider]; !exists {
				return fmt.Errorf("pkgbase %q depends on unknown planned pkgbase %q", current, provider)
			}
			if err := visit(provider); err != nil {
				return err
			}
			artifacts, exists := built[provider]
			if !exists {
				return fmt.Errorf("pkgbase %q dependency %q has not been built", base, provider)
			}
			for _, artifact := range artifacts.packages {
				paths = append(paths, artifact.path)
			}
			seen[provider] = struct{}{}
		}
		return nil
	}
	if err := visit(base); err != nil {
		return nil, err
	}
	slices.Sort(paths)
	return paths, nil
}

func validateBuildResult(result *builder.Result, source *sourceRecord, arch, buildInfoDir string) ([]builtPackage, error) {
	if result == nil {
		return nil, fmt.Errorf("backend returned no result")
	}
	expected := source.metadata.OutputNames(arch)
	seen := map[string]struct{}{}
	packages := make([]builtPackage, 0, len(result.Packages))
	if err := os.MkdirAll(buildInfoDir, 0o700); err != nil {
		return nil, err
	}
	for _, packagePath := range result.Packages {
		info, err := os.Lstat(packagePath)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("package artifact %q is not a regular file", packagePath)
		}
		metadata, err := pacmanpkg.ReadBinaryPackageMeta(packagePath)
		if err != nil {
			return nil, err
		}
		name := metadata.Info.PkgName
		if !slices.Contains(expected, name) && !slices.Contains(expected, strings.TrimSuffix(name, "-debug")) {
			return nil, fmt.Errorf("unexpected package %q; .SRCINFO declares %v", name, expected)
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("backend produced package %q more than once", name)
		}
		seen[name] = struct{}{}
		if metadata.Info.PkgVer != source.metadata.Version() {
			return nil, fmt.Errorf("package %q version %q differs from .SRCINFO version %q", name, metadata.Info.PkgVer, source.metadata.Version())
		}
		if metadata.Info.PkgBase != "" && metadata.Info.PkgBase != source.metadata.Base() {
			return nil, fmt.Errorf("package %q pkgbase %q differs from .SRCINFO pkgbase %q", name, metadata.Info.PkgBase, source.metadata.Base())
		}
		if metadata.Info.Arch != arch && metadata.Info.Arch != "any" {
			return nil, fmt.Errorf("package %q architecture %q differs from target %q", name, metadata.Info.Arch, arch)
		}
		artifact, err := pacmanpkg.ParseArtifact(filepath.Base(packagePath))
		if err != nil {
			return nil, err
		}
		coordinates, err := artifact.Coordinates()
		if err != nil {
			return nil, err
		}
		if !coordinates.MatchesMetadata(name, metadata.Info.PkgVer, metadata.Info.Arch) {
			return nil, fmt.Errorf("package filename %q differs from .PKGINFO", filepath.Base(packagePath))
		}
		file, err := os.Open(packagePath)
		if err != nil {
			return nil, err
		}
		buildInfo, readErr := pacmanpkg.ReadBuildInfoData(file)
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return nil, errors.Join(readErr, closeErr)
		}
		buildInfoPath := filepath.Join(buildInfoDir, name+".BUILDINFO")
		if err := safefile.WriteFile(buildInfoPath, buildInfo, 0o644); err != nil {
			return nil, err
		}
		signature := ""
		signatureInfo, err := os.Lstat(packagePath + ".sig")
		if err == nil {
			if !signatureInfo.Mode().IsRegular() {
				return nil, fmt.Errorf("package signature %q is not a regular file", packagePath+".sig")
			}
			signature = packagePath + ".sig"
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		_, explicit := source.explicit[name]
		packages = append(packages, builtPackage{
			path: packagePath, signature: signature, buildInfo: buildInfoPath,
			manifest: ManifestPackage{
				Name: name, Version: metadata.Info.PkgVer, Arch: metadata.Info.Arch,
				SHA256: metadata.SHA256, Explicit: explicit,
			},
		})
	}
	for _, name := range expected {
		if _, exists := seen[name]; !exists {
			return nil, fmt.Errorf(".SRCINFO declares package %q but the backend did not produce it", name)
		}
	}
	slices.SortFunc(packages, func(left, right builtPackage) int {
		return strings.Compare(left.manifest.Name, right.manifest.Name)
	})
	return packages, nil
}

func (s *Service) publish(request BuildRequest, plan *Plan, built map[string]builtSource, output, manifestPath string, externalLogs bool) (*Manifest, error) {
	staging, err := os.MkdirTemp(filepath.Dir(output), "."+filepath.Base(output)+".staging-*")
	if err != nil {
		return nil, fmt.Errorf("create repository staging directory: %w", err)
	}
	stagingExists := true
	defer func() {
		if stagingExists {
			_ = os.RemoveAll(staging)
		}
	}()

	manifest := &Manifest{
		SchemaVersion:    SchemaVersion,
		Arch:             request.Arch,
		Backend:          s.backend.Name(),
		BuildEnvironment: request.Environment,
		Repository: ManifestRepository{
			Name: request.RepoName, Path: output, Database: filepath.Join(output, request.RepoName+".db"),
		},
		Install: append([]string(nil), plan.Install...),
	}
	if reporter, ok := s.backend.(builder.EnvironmentReporter); ok {
		reported := reporter.BuildEnvironment()
		if reported.Image != "" {
			manifest.BuildEnvironment = reported
		}
	}

	var packagePaths []string
	filenames := map[string]string{}
	for _, planned := range plan.Builds {
		result := built[planned.Pkgbase]
		logPath := result.log
		if !externalLogs {
			logTarget := filepath.Join(staging, "logs", filepath.Base(result.log))
			if err := copyFile(result.log, logTarget); err != nil {
				return nil, err
			}
			logPath = filepath.Join(output, "logs", filepath.Base(result.log))
		}
		manifestBuild := ManifestBuild{
			Pkgbase: planned.Pkgbase, Source: planned.Source, Explicit: len(planned.Explicit) > 0,
			Log: logPath, Dependencies: append([]Dependency(nil), planned.Dependencies...),
		}
		for _, artifact := range result.packages {
			filename := filepath.Base(artifact.path)
			if previous, duplicate := filenames[filename]; duplicate {
				return nil, fmt.Errorf("package filename %q is produced by both %s and %s", filename, previous, planned.Pkgbase)
			}
			filenames[filename] = planned.Pkgbase
			packageTarget := filepath.Join(staging, filename)
			if err := copyFile(artifact.path, packageTarget); err != nil {
				return nil, err
			}
			packagePaths = append(packagePaths, packageTarget)
			published := artifact.manifest
			published.File = filepath.Join(output, filename)
			if artifact.signature != "" {
				if err := copyFile(artifact.signature, packageTarget+".sig"); err != nil {
					return nil, err
				}
				published.Signature = published.File + ".sig"
			}
			buildInfoTarget := filepath.Join(staging, "buildinfo", planned.Pkgbase, filepath.Base(artifact.buildInfo))
			if err := copyFile(artifact.buildInfo, buildInfoTarget); err != nil {
				return nil, err
			}
			published.BuildInfo = filepath.Join(output, "buildinfo", planned.Pkgbase, filepath.Base(artifact.buildInfo))
			manifestBuild.Packages = append(manifestBuild.Packages, published)
		}
		manifest.Builds = append(manifest.Builds, manifestBuild)
	}
	slices.Sort(packagePaths)
	databasePath := filepath.Join(staging, repo.Artifacts(request.RepoName).DatabaseArchive())
	if err := s.repoTool.RepoAddBatch(databasePath, packagePaths, false, nil); err != nil {
		return nil, fmt.Errorf("create temporary repository: %w", err)
	}
	for _, name := range append(repo.Artifacts(request.RepoName).Archives(), repo.Artifacts(request.RepoName).Aliases()...) {
		if info, err := os.Stat(filepath.Join(staging, name)); err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("temporary repository is missing %q", name)
		}
	}
	if err := safefile.SyncDirectory(staging); err != nil {
		return nil, err
	}
	if err := os.Rename(staging, output); err != nil {
		return nil, fmt.Errorf("publish temporary repository: %w", err)
	}
	stagingExists = false
	if err := safefile.SyncDirectory(filepath.Dir(output)); err != nil {
		_ = os.RemoveAll(output)
		return nil, err
	}

	if err := safefile.Replace(manifestPath, 0o644, func(writer io.Writer) error {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(manifest)
	}); err != nil {
		_ = os.RemoveAll(output)
		return nil, fmt.Errorf("write manifest: %w", err)
	}
	return manifest, nil
}

func copyFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %q: %w", source, err)
	}
	defer input.Close()
	return safefile.Replace(destination, 0o644, func(writer io.Writer) error {
		_, err := io.Copy(writer, input)
		return err
	})
}

func pathWithin(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func validatePathOutside(output, path, label string) error {
	if path == "" {
		return nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", label, err)
	}
	if pathWithin(output, absolute) {
		return fmt.Errorf("%s %q must be outside repository output %q", label, absolute, output)
	}
	return nil
}
