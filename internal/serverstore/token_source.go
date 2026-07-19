package serverstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/filelock"
)

// TokenSource supplies and refreshes stored Ayato credentials.
type TokenSource struct {
	mu       sync.Mutex
	server   string
	username string
	access   string
	refresh  func(context.Context, string, string) (client.TokenPair, error)
}

func NewTokenSource(endpoint *Endpoint) *TokenSource {
	return &TokenSource{
		server:   endpoint.URL,
		username: endpoint.Username,
		access:   endpoint.AccessToken,
		refresh:  refreshTokenPair,
	}
}

func (s *TokenSource) Token(context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.access, nil
}

func (s *TokenSource) RefreshIfCurrent(ctx context.Context, staleAccess string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.access != staleAccess {
		return nil
	}

	return withRefreshLock(ctx, s.server, func() error {
		snapshot, err := snapshotCredentialsForRefresh(s.server)
		if err != nil {
			return err
		}
		if snapshot.Access != staleAccess {
			s.access = snapshot.Access
			s.username = snapshot.Username
			return nil
		}

		refreshToken := snapshot.Refresh
		if refreshToken == "" {
			return errors.NewErr("no refresh token is stored")
		}
		pair, err := s.refresh(ctx, s.server, refreshToken)
		if err != nil {
			return err
		}
		username := pair.Login
		if username == "" {
			username = s.username
		}
		current, saved, err := SaveTokensIfCurrent(snapshot, username, pair.AccessToken, pair.RefreshToken)
		if err != nil {
			return err
		}
		if saved {
			s.access = pair.AccessToken
			s.username = username
		} else {
			s.access = current.Access
			s.username = current.Username
		}
		return nil
	})
}

func refreshTokenPair(ctx context.Context, server, refreshToken string) (client.TokenPair, error) {
	ayato, err := client.NewAyato(server, nil)
	if err != nil {
		return client.TokenPair{}, err
	}
	return ayato.RefreshAccessToken(ctx, refreshToken)
}

func withRefreshLock(ctx context.Context, server string, operation func() error) error {
	cacheDirectory, err := os.UserCacheDir()
	if err != nil {
		return errors.WrapErr(err, "locate user cache for refresh lock")
	}
	lockDirectory := filepath.Join(cacheDirectory, "kamisato", "locks")
	if err := os.MkdirAll(lockDirectory, 0o700); err != nil {
		return errors.WrapErr(err, "create refresh lock directory")
	}
	digest := sha256.Sum256([]byte(server))
	lockPath := filepath.Join(lockDirectory, hex.EncodeToString(digest[:])+".lock")
	lock, err := filelock.AcquireContext(ctx, lockPath, 0o600, 50*time.Millisecond)
	if err != nil {
		return errors.WrapErr(err, "acquire refresh lock")
	}
	defer func() { _ = lock.Release() }()
	return operation()
}
