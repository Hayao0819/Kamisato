// Package sqlkv implements kv.Store on top of a SQL database via GORM. It stores
// every entry in one generic table keyed by (namespace, key); expiry is enforced
// at read time by filtering on expires_at, so the abstraction works on backends
// without native TTL.
package sqlkv

import (
	"errors"
	"fmt"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/kv"
	"github.com/Hayao0819/Kamisato/internal/utils"
	_ "github.com/lib/pq"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Entry is the generic row backing every namespaced key-value pair. The 191-byte
// cap on the key columns keeps the composite primary key within MySQL's utf8mb4
// index-length limit. A nil ExpiresAt means the entry never expires.
type Entry struct {
	Namespace string `gorm:"primaryKey;size:191"`
	Key       string `gorm:"primaryKey;size:191"`
	Value     []byte
	ExpiresAt *time.Time
}

// Store is a kv.Store backed by a GORM-managed SQL database.
type Store struct {
	db *gorm.DB
}

// compile-time interface check.
var _ kv.Store = (*Store)(nil)

// New opens a SQL database with the given driver and dsn, migrates the generic
// Entry table, and returns a kv.Store.
func New(driver, dsn string) (*Store, error) {
	var dialector gorm.Dialector
	switch driver {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "mysql":
		dialector = mysql.Open(dsn)
	case "sqlite", "sqlite3":
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: utils.GormLog(),
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Entry{}); err != nil {
		return nil, fmt.Errorf("failed to auto migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// NewWithDB wraps an already-open *gorm.DB (migrating the Entry table). It is the
// seam tests use to inject an in-memory database.
func NewWithDB(db *gorm.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("sqlkv: nil db")
	}
	if err := db.AutoMigrate(&Entry{}); err != nil {
		return nil, fmt.Errorf("failed to auto migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// Get returns the value under (ns, key), or kv.ErrNotFound when the row is absent
// or has expired.
func (s *Store) Get(ns, key string) ([]byte, error) {
	if s.db == nil {
		return nil, errors.New("sqlkv: database connection is nil")
	}
	var e Entry
	err := s.db.
		Where("namespace = ? AND key = ? AND (expires_at IS NULL OR expires_at > ?)", ns, key, time.Now()).
		First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, kv.ErrNotFound
	}
	if err != nil {
		return nil, utils.WrapErr(err, "sqlkv: get")
	}
	return e.Value, nil
}

// Set upserts value under (ns, key). A positive ttl sets expires_at; ttl == 0
// clears it (no expiry).
func (s *Store) Set(ns, key string, value []byte, ttl time.Duration) error {
	if s.db == nil {
		return errors.New("sqlkv: database connection is nil")
	}
	var expiresAt *time.Time
	if ttl > 0 {
		t := time.Now().Add(ttl)
		expiresAt = &t
	}
	e := Entry{Namespace: ns, Key: key, Value: value, ExpiresAt: expiresAt}
	return s.db.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "namespace"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "expires_at"}),
		}).
		Create(&e).Error
}

// Delete removes (ns, key). Removing a missing row is not an error.
func (s *Store) Delete(ns, key string) error {
	if s.db == nil {
		return errors.New("sqlkv: database connection is nil")
	}
	return s.db.
		Where("namespace = ? AND key = ?", ns, key).
		Delete(&Entry{}).Error
}

// List returns every non-expired entry within ns.
func (s *Store) List(ns string) ([]kv.Entry, error) {
	if s.db == nil {
		return nil, errors.New("sqlkv: database connection is nil")
	}
	var rows []Entry
	err := s.db.
		Where("namespace = ? AND (expires_at IS NULL OR expires_at > ?)", ns, time.Now()).
		Find(&rows).Error
	if err != nil {
		return nil, utils.WrapErr(err, "sqlkv: list")
	}
	out := make([]kv.Entry, 0, len(rows))
	for _, r := range rows {
		out = append(out, kv.Entry{Key: r.Key, Value: r.Value})
	}
	return out, nil
}

// Close closes the underlying sql.DB.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
