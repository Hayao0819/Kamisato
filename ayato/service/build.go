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
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

// BuildPackage builds a package using ayaka in a Docker container
func (s *Service) BuildPackage(repo string, buildReq *domain.BuildRequest) error {
	slog.Info("starting package build", "repo", repo, "arch", buildReq.Arch)

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

	// Upload built packages to repository
	if err := s.uploadBuiltPackages(repo, outDir); err != nil {
		return fmt.Errorf("failed to upload built packages: %w", err)
	}

	slog.Info("package build completed successfully", "repo", repo)
	return nil
}

// buildInDocker executes ayaka build inside a Docker container
func (s *Service) buildInDocker(srcDir, outDir string, buildReq *domain.BuildRequest) error {
	ctx := context.Background()

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	// Pull ayaka image
	imageName := "ghcr.io/hayao0819/kamisato:latest"
	slog.Info("pulling docker image", "image", imageName)
	reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader) // Wait for pull to complete

	// Prepare build command using makepkg
	// We use makepkg directly as it's more flexible for single PKGBUILD builds
	// Note: makepkg cannot run as root, so we need to create a build user
	buildCmd := []string{
		"sh", "-c",
		`set -e
		useradd -m builduser || true
		chown -R builduser:builduser /build
		su builduser -c "cd /build/src && makepkg --syncdeps --noconfirm --clean && mv *.pkg.tar.* /build/out/"
		`,
	}

	// Create container config
	containerConfig := &container.Config{
		Image:      imageName,
		Cmd:        buildCmd,
		WorkingDir: "/build",
		Tty:        false,
		User:       "root", // Need root to create builduser
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

	// Create container
	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	containerID := resp.ID
	defer func() {
		// Clean up container
		timeout := 10
		cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
		cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
	}()

	// Start container
	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		// Always get container logs for debugging
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
	case <-time.After(30 * time.Minute): // Timeout after 30 minutes
		return fmt.Errorf("build timeout")
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
		// Check if it's a package file
		if !isPackageFile(name) {
			continue
		}

		// Skip signature files for now, we'll handle them separately
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

		// Create FileStream for the package
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

		// Upload using existing UploadFile service
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
