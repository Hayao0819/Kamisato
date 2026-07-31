package config

import "testing"

func TestStoreValidate(t *testing.T) {
	if err := (StoreConfig{DBType: "badgerdb", StorageType: "s3"}).Validate(); err != nil {
		t.Errorf("valid store rejected: %v", err)
	}
	if err := (StoreConfig{}).Validate(); err != nil {
		t.Errorf("empty store rejected: %v", err)
	}
	if err := (StoreConfig{DBType: "badger"}).Validate(); err == nil {
		t.Error("unknown db_type was accepted")
	}
	if err := (StoreConfig{StorageType: "local"}).Validate(); err == nil {
		t.Error("unknown storage_type was accepted")
	}
}

func TestStoreCheckStateless(t *testing.T) {
	if err := (StoreConfig{}).checkStateless(false); err != nil {
		t.Errorf("local backend off Cloud Run rejected: %v", err)
	}
	for _, store := range []StoreConfig{
		{StorageType: "s3"},
		{DBType: "badgerdb", StorageType: "s3"},
		{DBType: "cfkv", StorageType: "localfs"},
		{DBType: "cfkv"},
	} {
		if err := store.checkStateless(true); err == nil {
			t.Errorf("Cloud Run store accepted: %+v", store)
		}
	}
	for _, store := range []StoreConfig{
		{DBType: "cfkv", StorageType: "s3"},
		{DBType: "sql", StorageType: "s3"},
	} {
		if err := store.checkStateless(true); err != nil {
			t.Errorf("remote Cloud Run store rejected: %v", err)
		}
	}
}
