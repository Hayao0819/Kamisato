package conf

import "testing"

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

func TestMikoValidate(t *testing.T) {
	for _, ok := range []string{"", "container", "chroot", "bwrap"} {
		if err := (&MikoConfig{Executor: ok}).Validate(); err != nil {
			t.Errorf("executor %q should be valid: %v", ok, err)
		}
	}
	if err := (&MikoConfig{Executor: "docker"}).Validate(); err == nil {
		t.Error("expected an error for an unknown executor")
	}
}
