package conf

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type DbConfig struct {
	Driver        string `koanf:"driver"`         // postgres | mysql | sqlite
	Host          string `koanf:"host"`           // 例: localhost
	Port          string `koanf:"port"`           // 例: 5432, 3306
	User          string `koanf:"user"`           // 例: root
	Password      string `koanf:"password"`       // 任意（SQLite除く）
	Database      string `koanf:"database"`       // DB名 or SQLiteファイルパス
	AdditionalDSN string `koanf:"additional_dsn"` // 例: sslmode=require
}

func (c DbConfig) DSN() (string, error) {
	switch strings.ToLower(c.Driver) {
	case "postgres":
		return c.postgresDSN()
	case "mysql":
		return c.mysqlDSN()
	case "sqlite", "sqlite3":
		return c.sqliteDSN()
	default:
		return "", fmt.Errorf("unsupported database driver: %s", c.Driver)
	}
}

func (c DbConfig) postgresDSN() (string, error) {
	if c.Host == "" || c.User == "" || c.Database == "" {
		return "", errors.New("postgres: missing required fields (host, user, database)")
	}
	port := c.Port
	if port == "" {
		port = "5432"
	}

	base := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s",
		c.Host, port, c.User, c.Password, c.Database,
	)

	if c.AdditionalDSN != "" {
		return fmt.Sprintf("%s %s", base, strings.TrimSpace(c.AdditionalDSN)), nil
	}
	return base, nil
}

func (c DbConfig) mysqlDSN() (string, error) {
	if c.Host == "" || c.User == "" || c.Database == "" {
		return "", errors.New("mysql: missing required fields (host, user, database)")
	}
	port := c.Port
	if port == "" {
		port = "3306"
	}

	base := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s",
		c.User, c.Password, c.Host, port, c.Database,
	)

	if c.AdditionalDSN != "" {
		return fmt.Sprintf("%s?%s", base, strings.TrimPrefix(c.AdditionalDSN, "?")), nil
	}
	return base, nil
}

func (c DbConfig) sqliteDSN() (string, error) {
	if c.Database == "" {
		return "", errors.New("sqlite: missing required field: database (file path)")
	}
	path := filepath.Clean(c.Database)
	if c.AdditionalDSN != "" {
		return fmt.Sprintf("%s?%s", path, strings.TrimPrefix(c.AdditionalDSN, "?")), nil
	}
	return path, nil
}
