// Package serverstore owns Ayaka's Ayato endpoints and credentials.
package serverstore

import (
	"crypto/sha256"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/atomicfile"
)

var (
	ErrNoServerSpecified = errors.NewErr("no server specified and no default server is set")
	ErrServerNotFound    = errors.NewErr("server not found")
)

type Endpoint struct {
	URL         string
	Username    string
	AccessToken string
}

type EndpointSummary struct {
	URL      string
	Username string
	Default  bool
}

// CredentialSnapshot is a revisioned access and refresh token pair.
type CredentialSnapshot struct {
	Server   string
	Username string
	Access   string
	Refresh  string
	revision [sha256.Size]byte
}

func readDB() (blinkyutils.Registry, error) {
	db := blinkyutils.NewRegistry()
	path, err := blinkyutils.RegistryPath()
	if err != nil {
		return db, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return db, nil
	}
	if err != nil {
		return db, errors.WrapErr(err, "read server database")
	}
	return blinkyutils.DecodeRegistry(raw)
}

func saveDB(db blinkyutils.Registry) error {
	path, err := blinkyutils.RegistryPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.WrapErr(err, "create server database directory")
	}
	raw, err := blinkyutils.EncodeRegistry(db)
	if err != nil {
		return err
	}
	return errors.WrapErr(atomicfile.WriteFile(path, raw, 0o600), "save server database")
}

func ResolveName(name string) (string, error) {
	db, err := readDB()
	if err != nil {
		return "", err
	}
	if name == "" {
		name = db.Default
	}
	if name == "" {
		return "", ErrNoServerSpecified
	}
	return name, nil
}

func Resolve(name string) (*Endpoint, error) {
	db, err := readDB()
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = db.Default
	}
	if name == "" {
		return nil, ErrNoServerSpecified
	}
	entry, ok := db.Endpoints[name]
	if !ok {
		return nil, errors.WrapErr(
			ErrServerNotFound,
			"server "+name+" is not registered; log in first with 'ayaka server login "+name+"'",
		)
	}
	return &Endpoint{
		URL:         name,
		Username:    entry.Username,
		AccessToken: loadAccessTokenValue(name, entry.AccessToken),
	}, nil
}

func ListEndpoints() ([]EndpointSummary, error) {
	db, err := readDB()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(db.Endpoints))
	for name := range db.Endpoints {
		names = append(names, name)
	}
	sort.Strings(names)
	endpoints := make([]EndpointSummary, 0, len(names))
	for _, name := range names {
		entry := db.Endpoints[name]
		endpoints = append(endpoints, EndpointSummary{
			URL:      name,
			Username: entry.Username,
			Default:  db.Default == name,
		})
	}
	return endpoints, nil
}

func Names(prefix string) []string {
	endpoints, err := ListEndpoints()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		name := endpoint.URL
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	return names
}

// SaveEndpoint stores an endpoint without changing its credentials.
func SaveEndpoint(server, username string) error {
	return withCredentialMutation(func() error {
		db, err := readDB()
		if err != nil {
			return err
		}
		entry := db.Endpoints[server]
		if username != "" {
			entry.Username = username
		}
		db.Endpoints[server] = entry
		return saveDB(db)
	})
}

// SaveStaticToken replaces stored credentials with a static Bearer token.
func SaveStaticToken(server, username, access string) error {
	return withCredentialMutation(func() error {
		return saveStaticToken(server, username, access)
	})
}

func saveStaticToken(server, username, access string) error {
	db, err := readDB()
	if err != nil {
		return err
	}
	if db.Endpoints == nil {
		db.Endpoints = make(map[string]blinkyutils.StoredEndpoint)
	}
	entry := db.Endpoints[server]
	if username != "" {
		entry.Username = username
	}
	if access == "" {
		if err := saveCredentialMode(server, credentialMode{Access: credentialSourceNone, Refresh: credentialSourceNone}); err != nil {
			return err
		}
		entry.AccessToken = ""
		db.Endpoints[server] = entry
		if err := saveDB(db); err != nil {
			return err
		}
		forgetKeyringTokensBestEffort(server)
		return nil
	}
	if err := saveCredentialMode(server, credentialMode{Access: credentialSourceNone, Refresh: credentialSourceNone}); err != nil {
		return err
	}
	stored, err := storeAccessToken(server, access)
	if err != nil {
		return err
	}
	if stored {
		entry.AccessToken = ""
	} else {
		entry.AccessToken = access
	}
	db.Endpoints[server] = entry
	if err := saveDB(db); err != nil {
		return err
	}
	accessSource := credentialSourceFile
	if stored {
		accessSource = credentialSourceKeyring
	}
	if err := saveCredentialMode(server, credentialMode{Access: accessSource, Refresh: credentialSourceNone}); err != nil {
		return err
	}
	if err := forgetRefreshToken(server); err != nil {
		slog.Debug("could not remove inactive refresh token from OS keyring", "server", server, "err", err)
	}
	if !stored {
		if err := forgetAccessToken(server); err != nil {
			slog.Debug("could not remove inactive access token from OS keyring", "server", server, "err", err)
		}
	}
	return nil
}

// SaveTokens stores an access and refresh token pair.
func SaveTokens(server, username, access, refresh string) error {
	if access == "" || refresh == "" {
		return errors.NewErr("OAuth token pair must contain both access and refresh tokens")
	}
	return withCredentialMutation(func() error {
		return saveTokens(server, username, access, refresh)
	})
}

// SnapshotCredentials returns a consistent credential pair.
func SnapshotCredentials(server string) (CredentialSnapshot, error) {
	var snapshot CredentialSnapshot
	err := withCredentialMutation(func() error {
		db, err := readDB()
		if err != nil {
			return err
		}
		snapshot, err = credentialSnapshot(db, server)
		return err
	})
	return snapshot, err
}

// snapshotCredentialsForRefresh verifies refresh-token availability.
func snapshotCredentialsForRefresh(server string) (CredentialSnapshot, error) {
	var snapshot CredentialSnapshot
	err := withCredentialMutation(func() error {
		db, err := readDB()
		if err != nil {
			return err
		}
		snapshot, err = credentialSnapshot(db, server)
		if err != nil || snapshot.Refresh == "" {
			return err
		}
		stored, storeErr := storeRefreshToken(server, snapshot.Refresh)
		if storeErr != nil || !stored {
			return errors.NewErr("refresh token keyring is unavailable")
		}
		return nil
	})
	return snapshot, err
}

func credentialSnapshot(db blinkyutils.Registry, server string) (CredentialSnapshot, error) {
	entry, ok := db.Endpoints[server]
	if !ok {
		return CredentialSnapshot{}, errors.WrapErr(ErrServerNotFound, server)
	}
	access, err := loadAccessToken(server, entry.AccessToken)
	if err != nil {
		return CredentialSnapshot{}, err
	}
	refresh, err := loadRefreshToken(server)
	if err != nil {
		return CredentialSnapshot{}, err
	}
	snapshot := CredentialSnapshot{Server: server, Username: entry.Username, Access: access, Refresh: refresh}
	snapshot.revision = sha256.Sum256([]byte(server + "\x00" + access + "\x00" + refresh))
	return snapshot, nil
}

// SaveTokensIfCurrent stores a pair if the credential revision is unchanged.
func SaveTokensIfCurrent(expected CredentialSnapshot, username, access, refresh string) (current CredentialSnapshot, saved bool, err error) {
	err = withCredentialMutation(func() error {
		db, readErr := readDB()
		if readErr != nil {
			return readErr
		}
		current, readErr = credentialSnapshot(db, expected.Server)
		if readErr != nil {
			return readErr
		}
		if current.revision != expected.revision {
			return nil
		}
		if saveErr := saveTokens(expected.Server, username, access, refresh); saveErr != nil {
			return saveErr
		}
		db, readErr = readDB()
		if readErr != nil {
			return readErr
		}
		current, readErr = credentialSnapshot(db, expected.Server)
		if readErr != nil {
			return readErr
		}
		saved = true
		return nil
	})
	return current, saved, err
}

func saveTokens(server, username, access, refresh string) error {
	db, err := readDB()
	if err != nil {
		return err
	}
	if db.Endpoints == nil {
		db.Endpoints = make(map[string]blinkyutils.StoredEndpoint)
	}
	entry := db.Endpoints[server]
	if username != "" {
		entry.Username = username
	}
	if err := saveCredentialMode(server, credentialMode{Access: credentialSourceNone, Refresh: credentialSourceNone}); err != nil {
		return err
	}
	refreshStored, storeErr := storeRefreshToken(server, refresh)
	if storeErr != nil || !refreshStored {
		return errors.WrapErr(storeErr, "failed to persist refresh token in the OS keyring")
	}
	stored, err := storeAccessToken(server, access)
	if err != nil {
		return err
	}
	if stored {
		entry.AccessToken = ""
	} else {
		entry.AccessToken = access
	}
	db.Endpoints[server] = entry
	if err := saveDB(db); err != nil {
		return err
	}
	accessSource := credentialSourceFile
	if stored {
		accessSource = credentialSourceKeyring
	}
	return saveCredentialMode(server, credentialMode{Access: accessSource, Refresh: credentialSourceKeyring})
}

func ForgetTokens(server string) error {
	return withCredentialMutation(func() error {
		if err := saveCredentialMode(server, credentialMode{Access: credentialSourceNone, Refresh: credentialSourceNone}); err != nil {
			return err
		}
		forgetKeyringTokensBestEffort(server)
		return nil
	})
}

// ClearCredentials disables and removes stored credentials.
func ClearCredentials(server string, clearUsername bool) error {
	return withCredentialMutation(func() error {
		db, err := readDB()
		if err != nil {
			return err
		}
		return clearCredentials(db, server, clearUsername)
	})
}

// ClearCredentialsIfCurrent clears credentials if the revision is unchanged.
func ClearCredentialsIfCurrent(expected CredentialSnapshot, clearUsername bool) (bool, error) {
	cleared := false
	err := withCredentialMutation(func() error {
		db, err := readDB()
		if err != nil {
			return err
		}
		current, err := credentialSnapshot(db, expected.Server)
		if err != nil {
			return err
		}
		if current.revision != expected.revision {
			return nil
		}
		if err := clearCredentials(db, expected.Server, clearUsername); err != nil {
			return err
		}
		cleared = true
		return nil
	})
	return cleared, err
}

func clearCredentials(db blinkyutils.Registry, server string, clearUsername bool) error {
	entry, ok := db.Endpoints[server]
	if !ok {
		return errors.WrapErr(ErrServerNotFound, server)
	}
	if err := saveCredentialMode(server, credentialMode{Access: credentialSourceNone, Refresh: credentialSourceNone}); err != nil {
		return err
	}
	entry.AccessToken = ""
	if clearUsername {
		entry.Username = ""
	}
	db.Endpoints[server] = entry
	if err := saveDB(db); err != nil {
		return err
	}
	forgetKeyringTokensBestEffort(server)
	return nil
}

// RemoveEndpoint removes an endpoint and its credentials.
func RemoveEndpoint(server string) error {
	return withCredentialMutation(func() error {
		db, err := readDB()
		if err != nil {
			return err
		}
		if err := saveCredentialMode(server, credentialMode{Access: credentialSourceNone, Refresh: credentialSourceNone}); err != nil {
			return err
		}
		delete(db.Endpoints, server)
		if db.Default == server {
			db.Default = ""
		}
		if err := saveDB(db); err != nil {
			return err
		}
		forgetKeyringTokensBestEffort(server)
		return nil
	})
}

// SetDefault sets the default endpoint.
func SetDefault(server string) error {
	return withCredentialMutation(func() error {
		db, err := readDB()
		if err != nil {
			return err
		}
		if _, ok := db.Endpoints[server]; !ok {
			return errors.WrapErr(ErrServerNotFound, server)
		}
		db.Default = server
		return saveDB(db)
	})
}

func forgetKeyringTokensBestEffort(server string) {
	if err := forgetAccessToken(server); err != nil {
		slog.Debug("could not remove inactive access token from OS keyring", "server", server, "err", err)
	}
	if err := forgetRefreshToken(server); err != nil {
		slog.Debug("could not remove inactive refresh token from OS keyring", "server", server, "err", err)
	}
}
