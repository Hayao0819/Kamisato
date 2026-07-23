package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/internal/errors"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

type stagedFileRequest struct {
	Name string `json:"name"`
	Size int64  `json:"size,omitempty"`
}

type stagedUploadGrant struct {
	ID         string            `json:"id"`
	TTLSeconds int               `json:"ttl_seconds"`
	URLs       map[string]string `json:"urls"`
}

type stagedCommitEntry struct {
	Package   string `json:"package"`
	Signature string `json:"signature,omitempty"`
}

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

	// One request per package: a batch of large artifacts (kernel + headers)
	// exceeds proxy body limits (Cloudflare caps requests at 100MB). The staged
	// protocol sidesteps that cap entirely by PUTting bytes straight to storage;
	// once a server answers "unsupported" once, stop probing and use multipart
	// for the rest of the batch.
	stagedUnavailable := false
	for _, pkg := range packages {
		if !stagedUnavailable {
			fallback, err := uploadStagedPackage(ctx, requester, repo, pkg)
			if err != nil {
				return err
			}
			if !fallback {
				continue
			}
			stagedUnavailable = true
		}
		err := requester.execute(ctx, func() error {
			return uploadMultipart(
				ctx,
				requester.transport,
				requester.transport.endpoint("api", "unstable", "repos", repo, "packages"),
				[]string{pkg},
			)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// uploadStagedPackage presigns, PUTs, and commits one package (plus its
// sibling signature, if present) through the staging-intent protocol. fallback
// reports that the server has no staging capability, so the caller should use
// uploadMultipart instead — for this package and every remaining one.
func uploadStagedPackage(ctx context.Context, requester *requester, repo, pkg string) (fallback bool, err error) {
	names := []string{filepath.Base(pkg)}
	paths := map[string]string{names[0]: pkg}
	if signature := pkg + ".sig"; fileExists(signature) {
		names = append(names, filepath.Base(signature))
		paths[names[1]] = signature
	}

	reqFiles := make([]stagedFileRequest, 0, len(names))
	sizes := make(map[string]int64, len(names))
	for _, name := range names {
		info, statErr := os.Stat(paths[name])
		if statErr != nil {
			return false, errors.WrapErr(statErr, "stat "+paths[name])
		}
		sizes[name] = info.Size()
		reqFiles = append(reqFiles, stagedFileRequest{Name: name, Size: info.Size()})
	}

	var grant stagedUploadGrant
	err = requester.execute(ctx, func() error {
		return requester.transport.doJSON(
			ctx,
			noRetry,
			http.MethodPost,
			requester.transport.endpoint("api", "unstable", "repos", repo, "packages", "presign"),
			true,
			struct {
				Files []stagedFileRequest `json:"files"`
			}{Files: reqFiles},
			&grant,
			http.StatusOK,
			"presign package upload",
		)
	})
	if stagedProtocolUnavailable(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	for _, name := range names {
		rawURL, granted := grant.URLs[name]
		if !granted {
			return false, errors.NewErr("presign response is missing a URL for " + name)
		}
		if err := putStagedFile(ctx, requester.transport, rawURL, paths[name], sizes[name]); err != nil {
			return false, err
		}
	}

	entry := stagedCommitEntry{Package: names[0]}
	if len(names) > 1 {
		entry.Signature = names[1]
	}
	err = requester.execute(ctx, func() error {
		return commitStagedUpload(ctx, requester.transport, repo, grant.ID, entry)
	})
	return false, err
}

// commitStagedUpload deliberately skips the per-attempt timeout: server-side
// commit work scales with package size, and abandoning a commit that then
// succeeds would turn its retry into a version-gate rejection.
func commitStagedUpload(ctx context.Context, t *transport, repo, id string, entry stagedCommitEntry) error {
	payload, err := json.Marshal(struct {
		ID    string              `json:"id"`
		Files []stagedCommitEntry `json:"files"`
	}{ID: id, Files: []stagedCommitEntry{entry}})
	if err != nil {
		return errors.WrapErr(err, "encode commit request")
	}
	req, err := t.newRequest(
		ctx,
		http.MethodPost,
		t.endpoint("api", "unstable", "repos", repo, "packages", "commit"),
		bytes.NewReader(payload),
		true,
	)
	if err != nil {
		return errors.WrapErr(err, "create commit request")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.http.Do(req)
	if err != nil {
		return errors.WrapErr(err, "commit staged upload")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return responseError(resp, "commit staged upload")
	}
	return nil
}

// stagedProtocolUnavailable recognizes a server that either predates the
// staging-intent protocol (404) or has it compiled in but unsupported by its
// blob backend (501, the tombstone response).
func stagedProtocolUnavailable(err error) bool {
	var respErr *ResponseError
	if !errors.As(err, &respErr) {
		return false
	}
	return respErr.StatusCode == http.StatusNotImplemented || respErr.StatusCode == http.StatusNotFound
}

// putStagedFile streams path's bytes to a presigned URL with no credential:
// the URL itself is the authorization.
func putStagedFile(ctx context.Context, t *transport, rawURL, path string, size int64) error {
	target, err := url.Parse(rawURL)
	if err != nil {
		return errors.WrapErr(err, "parse presigned URL")
	}
	file, err := os.Open(path)
	if err != nil {
		return errors.WrapErr(err, "open "+path)
	}
	defer file.Close()

	req, err := t.newRequest(ctx, http.MethodPut, target, file, false)
	if err != nil {
		return errors.WrapErr(err, "create staged put request")
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	// The presigned URL signs this exact length; storage rejects a mismatch.
	req.ContentLength = size
	resp, err := t.http.Do(req)
	if err != nil {
		return errors.WrapErr(err, "put staged file "+filepath.Base(path))
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return responseError(resp, "put staged file "+filepath.Base(path))
	}
	return nil
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
