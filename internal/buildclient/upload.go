package buildclient

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
)

// UploadPackageFiles publishes package files to a repo, mirroring the GitHub
// Action: it requests a presigned direct-to-store PUT URL per file so a package
// larger than the server's request-body limit is uploaded without going through
// the server, then finalizes. A backend that cannot presign answers 501, and the
// call falls back to the classic multipart POST.
func UploadPackageFiles(ctx context.Context, base, token, repo string, files ...string) error {
	// The .sig sidecars ride their package; each package's signature is the
	// sibling "<pkg>.sig" when it exists on disk.
	var packages []string
	for _, f := range files {
		if filepath.Ext(f) == ".sig" {
			continue
		}
		packages = append(packages, f)
	}
	if len(packages) == 0 {
		return errors.NewErr("no package files to upload")
	}

	// Presign every package and its .sig by basename.
	var wanted []string
	uploads := make([]string, 0, len(packages)*2)
	for _, pkg := range packages {
		wanted = append(wanted, filepath.Base(pkg))
		uploads = append(uploads, pkg)
		if sig := pkg + ".sig"; fileExists(sig) {
			wanted = append(wanted, filepath.Base(sig))
			uploads = append(uploads, sig)
		}
	}

	pkgBase := endpoint(base, "/api/unstable/repos/"+url.PathEscape(repo)+"/packages")

	urls, supported, err := presignUploads(ctx, pkgBase, token, wanted)
	if err != nil {
		return err
	}
	if !supported {
		// Backend cannot presign; publish via the classic multipart POST.
		return uploadMultipart(ctx, pkgBase, token, packages)
	}

	for _, f := range uploads {
		put, ok := urls[filepath.Base(f)]
		if !ok || put == "" {
			return errors.NewErrf("no presigned URL returned for %s", filepath.Base(f))
		}
		if err := putFile(ctx, put, f); err != nil {
			return err
		}
	}

	names := make([]string, len(packages))
	for i, pkg := range packages {
		names[i] = filepath.Base(pkg)
	}
	body := map[string]any{"packages": names}
	return doJSON(ctx, http.MethodPost, pkgBase+"/finalize", token, body, nil, http.StatusOK, "finalize upload")
}

// presignUploads requests a PUT URL per basename. supported is false when the
// backend cannot presign (HTTP 501), signalling the caller to fall back to the
// multipart POST. This drives the request directly rather than via doJSON so the
// 501 status can be distinguished from a real failure.
func presignUploads(ctx context.Context, pkgBase, token string, files []string) (urls map[string]string, supported bool, err error) {
	encoded, err := json.Marshal(map[string]any{"files": files})
	if err != nil {
		return nil, false, errors.WrapErr(err, "failed to encode presign upload request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pkgBase+"/presign", bytes.NewReader(encoded))
	if err != nil {
		return nil, false, errors.WrapErr(err, "failed to create presign upload request")
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, false, errors.WrapErr(err, "failed to send presign upload request")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotImplemented {
		return nil, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, responseErr(resp, "presign upload")
	}
	var out struct {
		URLs map[string]string `json:"urls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, false, errors.WrapErr(err, "failed to decode presign upload response")
	}
	return out.URLs, true, nil
}

// putFile streams a file's bytes to its presigned URL with no Authorization
// header (the URL is already authorized) and requires a 2xx.
func putFile(ctx context.Context, put, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.WrapErr(err, "failed to open "+path)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return errors.WrapErr(err, "failed to stat "+path)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, put, f)
	if err != nil {
		return errors.WrapErr(err, "failed to create upload request for "+filepath.Base(path))
	}
	req.ContentLength = info.Size()

	resp, err := streamClient.Do(req)
	if err != nil {
		return errors.WrapErr(err, "failed to upload "+filepath.Base(path))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseErr(resp, "upload "+filepath.Base(path))
	}
	return nil
}

// uploadMultipart is the fallback: POST the packages (and any .sig sidecar) as a
// multipart form matching BatchUploadHandler ("package" parts, "signature" parts
// named "<pkg>.sig"). The body is streamed through an io.Pipe so a large upload
// is not buffered in memory.
func uploadMultipart(ctx context.Context, pkgBase, token string, packages []string) error {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		var werr error
		defer func() { _ = pw.CloseWithError(werr) }()
		for _, pkg := range packages {
			if werr = writePart(mw, "package", pkg); werr != nil {
				return
			}
			if sig := pkg + ".sig"; fileExists(sig) {
				if werr = writePart(mw, "signature", sig); werr != nil {
					return
				}
			}
		}
		werr = mw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pkgBase, pr)
	if err != nil {
		_ = pr.CloseWithError(err)
		return errors.WrapErr(err, "failed to create upload request")
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := streamClient.Do(req)
	if err != nil {
		return errors.WrapErr(err, "failed to upload packages")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return responseErr(resp, "upload packages")
	}
	return nil
}

// writePart streams path into a multipart field named by its basename.
func writePart(mw *multipart.Writer, field, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.WrapErr(err, "failed to open "+path)
	}
	defer f.Close()

	part, err := mw.CreateFormFile(field, filepath.Base(path))
	if err != nil {
		return errors.WrapErr(err, "failed to create form field for "+filepath.Base(path))
	}
	if _, err := io.Copy(part, f); err != nil {
		return errors.WrapErr(err, "failed to write "+filepath.Base(path))
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
