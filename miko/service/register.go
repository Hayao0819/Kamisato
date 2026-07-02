package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/httpx"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// RegisterWorkerCert registers this worker's signing cert with ayato so its
// host-signed packages verify; ayato accepts it only if it chains to a trusted
// master. It is best-effort at boot: the caller logs and continues on failure.
func RegisterWorkerCert(ctx context.Context, cfg *conf.MikoConfig, ks *sign.Keystore) error {
	cert, err := ks.WorkerCertArmored()
	if err != nil {
		return err
	}
	url := strings.TrimRight(cfg.Ayato.URL, "/") + "/api/unstable/auth/signers"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(cert))
	if err != nil {
		return err
	}
	if cfg.Ayato.Username != "" || cfg.Ayato.Password != "" {
		req.SetBasicAuth(cfg.Ayato.Username, cfg.Ayato.Password)
	}
	req.Header.Set("Content-Type", "application/pgp-keys")
	// A short per-attempt timeout keeps a hung ayato from stalling boot; the
	// retries ride out ayato still coming up alongside miko in the same stack.
	resp, err := httpx.New(10*time.Second, 2).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ayato register signer: status %d: %s", resp.StatusCode, string(body))
	}
	slog.Info("registered worker signing key with ayato", "url", url)
	return nil
}
