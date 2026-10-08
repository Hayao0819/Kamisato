package kv

import (
	"errors"
	"reflect"
	"testing"
)

type auditStore struct {
	keys    []string
	deleted []string
	err     error
}

func (s *auditStore) ForeignKeys() ([]string, error) { return s.keys, s.err }
func (s *auditStore) DeleteRawKeys(keys []string) error {
	s.deleted = append([]string(nil), keys...)
	return nil
}

func TestAuditForeignKeysPrunesOnlyAfterSuccessfulAudit(t *testing.T) {
	for _, prune := range []bool{false, true} {
		s := &auditStore{keys: []string{"foreign-a", "foreign-b"}}
		got, err := AuditForeignKeys(s, prune)
		if err != nil || !reflect.DeepEqual(got, s.keys) {
			t.Fatalf("audit = %v, %v", got, err)
		}
		if prune && !reflect.DeepEqual(s.deleted, s.keys) {
			t.Fatalf("deleted = %v, want %v", s.deleted, s.keys)
		}
		if !prune && len(s.deleted) != 0 {
			t.Fatalf("read-only audit deleted %v", s.deleted)
		}
	}
	wantErr := errors.New("audit failed")
	s := &auditStore{keys: []string{"foreign"}, err: wantErr}
	if _, err := AuditForeignKeys(s, true); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if len(s.deleted) != 0 {
		t.Fatal("failed audit reached the destructive operation")
	}
}
