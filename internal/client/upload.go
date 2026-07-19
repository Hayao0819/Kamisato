package client

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/internal/errors"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

func (c *Ayato) UploadPackageFiles(ctx context.Context, repo string, files ...string) error {
	return uploadPackageFiles(ctx, c.request, repo, files)
}

func (c *Publisher) UploadPackageFiles(ctx context.Context, repo string, files ...string) error {
	return uploadPackageFiles(ctx, c.request, repo, files)
}

// uploadPackageFiles uploads packages through the multipart endpoint.
func uploadPackageFiles(ctx context.Context, requester *requester, repo string, files []string) error {
	packages := make([]string, 0, len(files))
	packageSet := make(map[string]bool, len(files))
	for _, file := range files {
		artifact, err := pacmanpkg.ParseArtifact(filepath.Base(file))
		if err != nil {
			return errors.WrapErr(err, "invalid package artifact "+file)
		}
		if !artifact.IsSignature() {
			packages = append(packages, file)
			packageSet[filepath.Clean(file)] = true
		}
	}
	if len(packages) == 0 {
		return errors.NewErr("no package files to upload")
	}
	for _, file := range files {
		artifact, err := pacmanpkg.ParseArtifact(filepath.Base(file))
		if err != nil {
			return errors.WrapErr(err, "invalid package artifact "+file)
		}
		if !artifact.IsSignature() {
			continue
		}
		archivePath := filepath.Join(filepath.Dir(file), artifact.ArchiveFilename())
		if !packageSet[filepath.Clean(archivePath)] {
			return errors.NewErr("signature has no matching package: " + file)
		}
		if !fileExists(file) {
			return errors.NewErr("signature file does not exist: " + file)
		}
	}

	return requester.execute(ctx, func() error {
		return uploadMultipart(
			ctx,
			requester.transport,
			requester.transport.endpoint("api", "unstable", "repos", repo, "packages"),
			packages,
		)
	})
}

// uploadMultipart creates a fresh multipart request body.
func uploadMultipart(ctx context.Context, transport *transport, targetURL *url.URL, packages []string) error {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	done := make(chan error, 1)
	go func() {
		var writeErr error
		defer func() {
			if closeErr := multipartWriter.Close(); writeErr == nil {
				writeErr = closeErr
			}
			_ = writer.CloseWithError(writeErr)
			done <- writeErr
		}()
		for _, path := range packages {
			if writeErr = writeMultipartPart(multipartWriter, "package", path); writeErr != nil {
				return
			}
			if signature := path + ".sig"; fileExists(signature) {
				if writeErr = writeMultipartPart(multipartWriter, "signature", signature); writeErr != nil {
					return
				}
			}
		}
	}()

	req, err := transport.newRequest(ctx, http.MethodPost, targetURL, reader, true)
	if err != nil {
		_ = reader.CloseWithError(err)
		<-done
		return errors.WrapErr(err, "create package upload request")
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	resp, err := transport.http.Do(req)
	if err != nil {
		_ = reader.CloseWithError(err)
		<-done
		return errors.WrapErr(err, "upload packages")
	}
	defer resp.Body.Close()
	_ = reader.Close()
	writeErr := <-done
	if resp.StatusCode != http.StatusOK {
		return responseError(resp, "upload packages")
	}
	if writeErr != nil {
		return errors.WrapErr(writeErr, "stream package upload")
	}
	return nil
}

func writeMultipartPart(writer *multipart.Writer, field, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return errors.WrapErr(err, "open "+path)
	}
	defer file.Close()

	part, err := writer.CreateFormFile(field, filepath.Base(path))
	if err != nil {
		return errors.WrapErr(err, "create multipart field for "+filepath.Base(path))
	}
	if _, err := io.Copy(part, file); err != nil {
		return errors.WrapErr(err, "write "+filepath.Base(path))
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
