package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/pkg/pacman/gpg"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// archToPlatform maps Arch Linux architecture names to Docker platform specs.
func archToPlatform(arch string) (*ocispec.Platform, error) {
	switch arch {
	case "x86_64":
		return &ocispec.Platform{OS: "linux", Architecture: "amd64"}, nil
	case "aarch64":
		return &ocispec.Platform{OS: "linux", Architecture: "arm64"}, nil
	case "armv7h":
		return &ocispec.Platform{OS: "linux", Architecture: "arm", Variant: "v7"}, nil
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", arch)
	}
}

// BuildPackage builds a package using makepkg in a Docker container
func (s *Service) BuildPackage(repo string, buildReq *domain.BuildRequest) error {
	slog.Info("starting package build", "repo", repo, "arch", buildReq.Arch)

	// Validate GPG configuration
	if buildReq.GPGKey != "" && s.cfg.Build.GnupgHome == "" {
		return fmt.Errorf("gpg key specified but gnupg_home is not configured")
	}

	// Create temporary directory for build
	tmpDir, err := os.MkdirTemp("", "ayaka-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory structure
	srcDir := filepath.Join(tmpDir, "src")
	outDir := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("failed to create src dir: %w", err)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create out dir: %w", err)
	}

	// Save PKGBUILD file
	pkgbuildPath := filepath.Join(srcDir, "PKGBUILD")
	pkgbuildFile, err := os.Create(pkgbuildPath)
	if err != nil {
		return fmt.Errorf("failed to create PKGBUILD file: %w", err)
	}
	if _, err := io.Copy(pkgbuildFile, buildReq.PKGBUILD); err != nil {
		pkgbuildFile.Close()
		return fmt.Errorf("failed to write PKGBUILD: %w", err)
	}
	pkgbuildFile.Close()

	// Save additional files if any
	for _, file := range buildReq.AdditionalFiles {
		destPath := filepath.Join(srcDir, file.FileName())
		destFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", file.FileName(), err)
		}
		if _, err := io.Copy(destFile, file); err != nil {
			destFile.Close()
			return fmt.Errorf("failed to write file %s: %w", file.FileName(), err)
		}
		destFile.Close()
	}

	// Build package using Docker
	if err := s.buildInDocker(srcDir, outDir, buildReq); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	// Sign built packages if GPG key is specified
	if buildReq.GPGKey != "" {
		if err := s.signBuiltPackages(outDir, buildReq.GPGKey); err != nil {
			return fmt.Errorf("failed to sign packages: %w", err)
		}
	}

	// Upload built packages to repository
	if err := s.uploadBuiltPackages(repo, outDir); err != nil {
		return fmt.Errorf("failed to upload built packages: %w", err)
	}

	slog.Info("package build completed successfully", "repo", repo)
	return nil
}

// buildInDocker executes makepkg inside a Docker container
func (s *Service) buildInDocker(srcDir, outDir string, buildReq *domain.BuildRequest) error {
	ctx := context.Background()

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	// Resolve image name from config
	imageName := s.cfg.Build.Image
	if imageName == "" {
		imageName = "archlinux:latest"
	}

	// Resolve platform from arch
	platform, err := archToPlatform(buildReq.Arch)
	if err != nil {
		return fmt.Errorf("failed to resolve platform: %w", err)
	}
	platformStr := platform.OS + "/" + platform.Architecture
	if platform.Variant != "" {
		platformStr += "/" + platform.Variant
	}

	slog.Info("pulling docker image", "image", imageName, "platform", platformStr)
	reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{Platform: platformStr})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader) // Wait for pull to complete

	// Generate makepkg.conf with correct CARCH
	makepkgConf := fmt.Sprintf("CARCH=\"%s\"\nCHOST=\"%s-pc-linux-gnu\"\n", buildReq.Arch, buildReq.Arch)
	makepkgConfPath := filepath.Join(srcDir, "makepkg.build.conf")
	if err := os.WriteFile(makepkgConfPath, []byte(makepkgConf), 0644); err != nil {
		return fmt.Errorf("failed to write makepkg.conf: %w", err)
	}

	// Prepare build command
	buildCmd := []string{
		"sh", "-c",
		`set -e
pacman -Syu --noconfirm base-devel
useradd -m builduser || true
chown -R builduser:builduser /build
su builduser -c "cd /build/src && makepkg --config /build/src/makepkg.build.conf --syncdeps --noconfirm --clean && mv *.pkg.tar.* /build/out/"
`,
	}

	// Create container config
	containerConfig := &container.Config{
		Image:      imageName,
		Cmd:        buildCmd,
		WorkingDir: "/build",
		Tty:        false,
		User:       "root",
	}

	// Create host config with volume mounts
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: srcDir,
				Target: "/build/src",
			},
			{
				Type:   mount.TypeBind,
				Source: outDir,
				Target: "/build/out",
			},
		},
	}

	// Create container with platform spec
	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, platform, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	containerID := resp.ID
	defer func() {
		timeout := 10
		cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
		cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
	}()

	// Start container
	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Resolve timeout
	buildTimeout := time.Duration(s.cfg.Build.Timeout) * time.Minute
	if buildTimeout <= 0 {
		buildTimeout = 30 * time.Minute
	}

	// Wait for container to finish
	statusCh, errCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		logs, logsErr := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
		})
		if logsErr == nil && logs != nil {
			defer logs.Close()
			logBytes, _ := io.ReadAll(logs)
			logStr := string(logBytes)
			slog.Info("build container logs", "logs", logStr)

			if status.StatusCode != 0 {
				slog.Error("build failed", "exit_code", status.StatusCode)
				return fmt.Errorf("build failed with exit code %d: %s", status.StatusCode, logStr)
			}
		} else if status.StatusCode != 0 {
			return fmt.Errorf("build failed with exit code %d", status.StatusCode)
		}
	case <-time.After(buildTimeout):
		return fmt.Errorf("build timeout after %v", buildTimeout)
	}

	return nil
}

// signBuiltPackages signs all built package files using GPG
func (s *Service) signBuiltPackages(outDir, gpgKey string) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("failed to read output directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isPackageFile(name) || filepath.Ext(name) == ".sig" {
			continue
		}

		pkgPath := filepath.Join(outDir, name)
		slog.Info("signing package", "file", name, "key", gpgKey)
		if err := gpg.SignFile(gpgKey, s.cfg.Build.GnupgHome, pkgPath); err != nil {
			return fmt.Errorf("failed to sign %s: %w", name, err)
		}
	}

	return nil
}

// uploadBuiltPackages uploads all .pkg.tar.* files from the output directory
func (s *Service) uploadBuiltPackages(repo, outDir string) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("failed to read output directory: %w", err)
	}

	uploadedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !isPackageFile(name) {
			continue
		}

		// Skip signature files, they are handled alongside their package
		if filepath.Ext(name) == ".sig" {
			continue
		}

		// Open package file
		pkgPath := filepath.Join(outDir, name)
		pkgFile, err := os.Open(pkgPath)
		if err != nil {
			slog.Error("failed to open package file", "file", name, "error", err)
			continue
		}

		pkgStream := stream.NewFileStream(name, "application/octet-stream", pkgFile)

		// Check for signature file
		var sigStream *stream.FileStream
		sigPath := pkgPath + ".sig"
		if _, err := os.Stat(sigPath); err == nil {
			sigFile, err := os.Open(sigPath)
			if err == nil {
				sigStream = stream.NewFileStream(name+".sig", "application/octet-stream", sigFile)
			}
		}

		files := &domain.UploadFiles{
			PkgFile: pkgStream,
			SigFile: sigStream,
		}

		if err := s.UploadFile(repo, files); err != nil {
			pkgFile.Close()
			if sigStream != nil {
				sigStream.Close()
			}
			return fmt.Errorf("failed to upload package %s: %w", name, err)
		}

		pkgFile.Close()
		if sigStream != nil {
			sigStream.Close()
		}

		uploadedCount++
		slog.Info("uploaded built package", "file", name, "repo", repo)
	}

	if uploadedCount == 0 {
		return fmt.Errorf("no package files found in output directory")
	}

	return nil
}

// isPackageFile checks if a file is an Arch Linux package
func isPackageFile(name string) bool {
	extensions := []string{".pkg.tar.zst", ".pkg.tar.xz", ".pkg.tar.gz", ".pkg.tar.bz2"}
	for _, ext := range extensions {
		if len(name) > len(ext) && name[len(name)-len(ext):] == ext {
			return true
		}
	}
	return false
}
