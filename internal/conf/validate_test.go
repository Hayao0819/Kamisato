package conf

import (
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

func TestStoreValidate(t *testing.T) {
	if err := (StoreConfig{DBType: "badgerdb", StorageType: "s3"}).Validate(); err != nil {
		t.Errorf("valid store rejected: %v", err)
	}
	if err := (StoreConfig{}).Validate(); err != nil {
		t.Errorf("empty store (defaults) rejected: %v", err)
	}
	if err := (StoreConfig{DBType: "badger"}).Validate(); err == nil {
		t.Error("expected an error for the typo db_type 'badger'")
	}
	if err := (StoreConfig{StorageType: "local"}).Validate(); err == nil {
		t.Error("expected an error for the typo storage_type 'local'")
	}
}

func TestStoreCheckStateless(t *testing.T) {
	// Not under Cloud Run: a local-disk backend is fine.
	if err := (StoreConfig{}).checkStateless(false); err != nil {
		t.Errorf("local backend off Cloud Run rejected: %v", err)
	}
	// Under Cloud Run: local disk on either axis must be rejected.
	if err := (StoreConfig{StorageType: "s3"}).checkStateless(true); err == nil {
		t.Error("expected rejection of the default (badgerdb) db_type under Cloud Run")
	}
	if err := (StoreConfig{DBType: "badgerdb", StorageType: "s3"}).checkStateless(true); err == nil {
		t.Error("expected rejection of badgerdb under Cloud Run")
	}
	if err := (StoreConfig{DBType: "cfkv", StorageType: "localfs"}).checkStateless(true); err == nil {
		t.Error("expected rejection of localfs under Cloud Run")
	}
	if err := (StoreConfig{DBType: "cfkv"}).checkStateless(true); err == nil {
		t.Error("expected rejection of the default (localfs) storage_type under Cloud Run")
	}
	// Under Cloud Run with remote backends on both axes: accepted.
	if err := (StoreConfig{DBType: "cfkv", StorageType: "s3"}).checkStateless(true); err != nil {
		t.Errorf("cfkv+s3 under Cloud Run rejected: %v", err)
	}
	if err := (StoreConfig{DBType: "sql", StorageType: "s3"}).checkStateless(true); err != nil {
		t.Errorf("sql+s3 under Cloud Run rejected: %v", err)
	}
}

func TestMikoValidate(t *testing.T) {
	for _, ok := range []string{"", "container", "chroot"} {
		if err := (&MikoConfig{Executor: ok}).Validate(); err != nil {
			t.Errorf("executor %q should be valid: %v", ok, err)
		}
	}
	if err := (&MikoConfig{Builder: builder.HostConfig{
		Backend: builder.KindBwrap,
		Bwrap:   builder.BwrapConfig{Rootfs: "/srv/arch-rootfs"},
	}}).Validate(); err != nil {
		t.Errorf("configured bwrap executor should be valid: %v", err)
	}
	if err := (&MikoConfig{Executor: "bwrap"}).Validate(); err == nil {
		t.Error("bwrap without builder.bwrap.rootfs should be rejected")
	}
	if err := (&MikoConfig{Executor: "docker"}).Validate(); err == nil {
		t.Error("expected an error for an unknown executor")
	}
}
