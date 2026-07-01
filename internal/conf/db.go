package conf

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-sql-driver/mysql"
)

type SqlConfig struct {
	Driver        string `koanf:"driver"`         // postgres | mysql | sqlite
	Host          string `koanf:"host"`           // e.g. localhost
	Port          string `koanf:"port"`           // e.g. 5432, 3306
	User          string `koanf:"user"`           // e.g. root
	Password      string `koanf:"password"`       // optional (except SQLite)
	Database      string `koanf:"database"`       // DB name or SQLite file path
	AdditionalDSN string `koanf:"additional_dsn"` // e.g. sslmode=require
}

func (c SqlConfig) DSN() (string, error) {
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

func (c SqlConfig) postgresDSN() (string, error) {
	if c.Host == "" || c.User == "" || c.Database == "" {
		return "", errors.New("postgres: missing required fields (host, user, database)")
	}
	port := c.Port
	if port == "" {
		port = "5432"
	}

	base := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s",
		pqQuote(c.Host), pqQuote(port), pqQuote(c.User), pqQuote(c.Password), pqQuote(c.Database),
	)

	if c.AdditionalDSN != "" {
		return fmt.Sprintf("%s %s", base, strings.TrimSpace(c.AdditionalDSN)), nil
	}
	return base, nil
}

// pqQuote quotes a libpq keyword/value connection-string value so that a secret
// containing whitespace, a quote or a backslash cannot corrupt the DSN. libpq
// treats every ASCII whitespace byte as a value separator, so all are triggers.
func pqQuote(v string) string {
	if !strings.ContainsAny(v, " \t\n\r\f\v'\\") {
		return v
	}
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
}

func (c SqlConfig) mysqlDSN() (string, error) {
	if c.Host == "" || c.User == "" || c.Database == "" {
		return "", errors.New("mysql: missing required fields (host, user, database)")
	}
	port := c.Port
	if port == "" {
		port = "3306"
	}

	// go-sql-driver's Config.FormatDSN emits a DSN its own ParseDSN round-trips, so
	// a password with @, :, / or other reserved bytes does not break parsing.
	cfg := mysql.NewConfig()
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, port)
	cfg.DBName = c.Database
	if c.AdditionalDSN != "" {
		params, err := url.ParseQuery(strings.TrimPrefix(c.AdditionalDSN, "?"))
		if err != nil {
			return "", fmt.Errorf("mysql: invalid additional_dsn: %w", err)
		}
		cfg.Params = make(map[string]string, len(params))
		for k := range params {
			cfg.Params[k] = params.Get(k)
		}
	}
	return cfg.FormatDSN(), nil
}

func (c SqlConfig) sqliteDSN() (string, error) {
	if c.Database == "" {
		return "", errors.New("sqlite: missing required field: database (file path)")
	}
	path := filepath.Clean(c.Database)
	if c.AdditionalDSN != "" {
		return fmt.Sprintf("%s?%s", path, strings.TrimPrefix(c.AdditionalDSN, "?")), nil
	}
	return path, nil
}
