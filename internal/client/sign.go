package client

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

const (
	SignPath          = "/api/unstable/sign"
	maxSignatureBytes = 16 << 20
)

// SignFile requests a detached signature.
func (c *Signer) SignFile(ctx context.Context, packagePath string) ([]byte, error) {
	packageFile, err := os.Open(packagePath)
	if err != nil {
		return nil, errors.WrapErr(err, "remote signer: open package")
	}
	defer packageFile.Close()
	info, err := packageFile.Stat()
	if err != nil {
		return nil, errors.WrapErr(err, "remote signer: stat package")
	}

	attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	request, err := c.request.transport.newRequest(
		attemptCtx,
		http.MethodPost,
		c.request.transport.endpoint("api", "unstable", "sign"),
		packageFile,
		true,
	)
	if err != nil {
		return nil, errors.WrapErr(err, "remote signer: create request")
	}
	request.ContentLength = info.Size()
	request.Header.Set("Content-Type", "application/octet-stream")

	response, err := c.request.transport.http.Do(request)
	if err != nil {
		return nil, errors.WrapErr(err, "remote signer: request")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, responseError(response, "remote signer")
	}
	if response.ContentLength > maxSignatureBytes {
		return nil, errors.NewErr("remote signer: signature response too large")
	}
	signature, err := io.ReadAll(io.LimitReader(response.Body, maxSignatureBytes+1))
	if err != nil {
		return nil, errors.WrapErr(err, "remote signer: read signature")
	}
	if len(signature) > maxSignatureBytes {
		return nil, errors.NewErr("remote signer: signature response too large")
	}
	if len(signature) == 0 {
		return nil, errors.NewErr("remote signer: empty signature")
	}
	return signature, nil
}
