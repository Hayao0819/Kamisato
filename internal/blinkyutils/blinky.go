package blinkyutils

import (
	"os"

	blinky_clientlib "github.com/BrenekH/blinky/clientlib"
	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// Sentinel errors for server resolution. Package-level so callers can errors.Is
// them through errwrap.WrapErr.
var (
	ErrNoServerSpecified = errwrap.NewErr("no server specified and no default server is set")
	ErrServerNotFound    = errwrap.NewErr("server not found")
)

// Client is the blinky upload client. Aliased so callers depend on blinkyutils
// instead of importing blinky's clientlib directly.
type Client = blinky_clientlib.BlinkyClient

// ServerInfo is a resolved ayato endpoint: the base URL plus the credentials
// stored in the blinky server database.
type ServerInfo struct {
	URL      string
	Username string
	Password string
}

// ResolveServerName returns the server to use: name, else the serverdb default.
// It does not require the server to be registered, so a bare URL works for the
// public job endpoints that need no credentials.
func ResolveServerName(name string) (string, error) {
	db, err := blinky_util.ReadServerDB()
	if err != nil {
		return "", errwrap.WrapErr(err, "failed to read server database")
	}
	if name == "" {
		name = db.DefaultServer
	}
	if name == "" {
		return "", ErrNoServerSpecified
	}
	return name, nil
}

// ResolveServer looks up the base URL and credentials in the serverdb, using the
// default server when name is empty. This is the same store blinky uploads use,
// so a server from `ayaka server login` works here too.
func ResolveServer(name string) (*ServerInfo, error) {
	db, err := blinky_util.ReadServerDB()
	if err != nil {
		return nil, errwrap.WrapErr(err, "failed to read server database")
	}
	if name == "" {
		name = db.DefaultServer
	}
	if name == "" {
		return nil, ErrNoServerSpecified
	}
	entry, ok := db.Servers[name]
	if !ok {
		return nil, errwrap.WrapErr(ErrServerNotFound,
			"server "+name+" is not registered; log in first with 'ayaka server login "+name+"'")
	}
	return &ServerInfo{URL: name, Username: entry.Username, Password: LoadSecret(name, entry.Password)}, nil
}

// Client builds a blinky client for the resolved endpoint.
func (s *ServerInfo) Client() (*Client, error) {
	return blinky_clientlib.New(s.URL, s.Username, s.Password)
}

// Upload sends a package file with its optional detached signature to repo. An
// empty sigPath uploads the package without a signature.
func Upload(client *Client, repo, pkgPath, sigPath string) error {
	pkgFile, err := os.Open(pkgPath)
	if err != nil {
		return err
	}
	defer func() { _ = pkgFile.Close() }()

	var sigFile *os.File
	if sigPath != "" {
		sigFile, err = os.Open(sigPath)
		if err != nil {
			return err
		}
		defer func() { _ = sigFile.Close() }()
	}

	// A nil signature reader tells the blinky client there is no signature.
	if sigFile == nil {
		return client.UploadPackage(repo, pkgPath, pkgFile, nil)
	}
	return client.UploadPackage(repo, pkgPath, pkgFile, sigFile)
}
