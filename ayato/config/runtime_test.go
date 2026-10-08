package config_test

import (
	"path/filepath"
	"testing"

	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
)

func TestRuntimeSettings(t *testing.T) {
	cfg := &ayatoconfig.AyatoConfig{
		RequireSign:                true,
		RequireBuildinfoProvenance: true,
		ProtectedNames:             []string{"pacman"},
		MaxBatchPackages:           4,
		MaxSize:                    1024,
		Repos: []ayatoconfig.BinRepoConfig{{
			Name:   "extra",
			Arches: []string{"x86_64"},
		}},
	}
	cfg.Store.BadgerDB = t.TempDir()
	cfg.Store.LocalRepoDir = t.TempDir()
	cfg.Verify.Keyring = "/tmp/keyring"

	repository, err := ayatoconfig.RepositorySettings(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if repository.KV.BadgerPath != filepath.Join(cfg.Store.BadgerDB, "kv-db") {
		t.Fatalf("badger path = %q", repository.KV.BadgerPath)
	}
	if names := repository.Catalog.PhysicalNames(); len(names) != 1 || names[0] != "extra" {
		t.Fatalf("repository names = %v", names)
	}

	service := ayatoconfig.ServiceSettings(cfg)
	if !service.RequireSign || !service.RequireBuildinfoProvenance {
		t.Fatalf("service policy = %+v", service)
	}
	if service.ExpectedBuildDir != "/build" || service.MaxPackageSize != 1024 {
		t.Fatalf("service settings = %+v", service)
	}
}

func TestRepositorySettingsRejectsInvalidSQL(t *testing.T) {
	cfg := &ayatoconfig.AyatoConfig{}
	cfg.Store.DBType = "sql"
	cfg.Store.SQL.Driver = "postgres"

	if _, err := ayatoconfig.RepositorySettings(cfg); err == nil {
		t.Fatal("RepositorySettings returned nil error")
	}
}
