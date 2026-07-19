package repository

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func badgerConfig(t *testing.T) *conf.AyatoConfig {
	t.Helper()
	cfg := &conf.AyatoConfig{}
	cfg.Store.BadgerDB = t.TempDir()
	cfg.Store.LocalRepoDir = t.TempDir()
	return cfg
}

func assertBadgerCanReopen(t *testing.T, cfg *conf.AyatoConfig) {
	t.Helper()
	store, err := badgerkv.New(cfg.DbPath())
	if err != nil {
		t.Fatalf("BadgerDB remained locked after constructor failure: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close reopened BadgerDB: %v", err)
	}
}

func TestNewClosesKVWhenSecretDecoratorInitializationFails(t *testing.T) {
	t.Setenv("AYATO_SECRETS_AGE_IDENTITY", "")
	cfg := badgerConfig(t)
	cfg.Secrets.AgeIdentityFile = filepath.Join(t.TempDir(), "missing.age")

	if _, _, _, _, err := New(cfg); err == nil {
		t.Fatal("New() = nil error with a missing age identity file")
	}
	assertBadgerCanReopen(t, cfg)
}

func TestNewClosesKVWhenLaterRepositoryInitializationFails(t *testing.T) {
	t.Setenv("AYATO_DB_SIGNING_KEY", "")
	cfg := badgerConfig(t)
	cfg.Sign.DB = true

	if _, _, _, _, err := New(cfg); err == nil {
		t.Fatal("New() = nil error without the configured database signing key")
	}
	assertBadgerCanReopen(t, cfg)
}

func TestNewMigrationStoresClosesKVOnFailure(t *testing.T) {
	t.Setenv("AYATO_SECRETS_AGE_IDENTITY", "")
	cfg := badgerConfig(t)
	cfg.Secrets.AgeIdentityFile = filepath.Join(t.TempDir(), "missing.age")

	if _, _, err := NewMigrationStores(cfg); err == nil {
		t.Fatal("NewMigrationStores() = nil error with a missing age identity file")
	}
	assertBadgerCanReopen(t, cfg)
}

type closeErrorStore struct {
	closeErr error
	closed   bool
}

func (*closeErrorStore) Get(string, string) ([]byte, error)              { return nil, kv.ErrNotFound }
func (*closeErrorStore) Set(string, string, []byte, time.Duration) error { return nil }
func (*closeErrorStore) Delete(string, string) error                     { return nil }
func (*closeErrorStore) List(string) ([]kv.Entry, error)                 { return nil, nil }
func (s *closeErrorStore) Close() error                                  { s.closed = true; return s.closeErr }

func TestCloseKVOnFailureKeepsPrimaryAndCloseErrors(t *testing.T) {
	primary := errors.New("initialize")
	closeErr := errors.New("close")
	store := &closeErrorStore{closeErr: closeErr}
	result := primary

	closeKVOnFailure(store, &result)

	if !store.closed {
		t.Fatal("store was not closed")
	}
	if !errors.Is(result, primary) || !errors.Is(result, closeErr) {
		t.Fatalf("result %v does not retain both primary and close errors", result)
	}
}
