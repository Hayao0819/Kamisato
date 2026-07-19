package serverstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// credentialMode selects the active credential sources.
type credentialMode struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
}

const (
	credentialSourceKeyring = "keyring"
	credentialSourceFile    = "file"
	credentialSourceNone    = "none"
)

type credentialState struct {
	Version int                       `json:"version"`
	Servers map[string]credentialMode `json:"servers"`
}

func credentialStatePath() (string, error) {
	dir, err := serverDataDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credential-state.json"), nil
}

func serverDataDirectory() (string, error) {
	return blinkyutils.DataDirectory()
}

var credentialMutationMu sync.Mutex

// withCredentialMutation serializes credential state changes.
func withCredentialMutation(operation func() error) error {
	credentialMutationMu.Lock()
	defer credentialMutationMu.Unlock()
	dir, err := serverDataDirectory()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.WrapErr(err, "create server data directory")
	}
	lock, err := os.OpenFile(filepath.Join(dir, "credentials.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return errors.WrapErr(err, "open credential mutation lock")
	}
	defer func() { _ = lock.Close() }()
	lockFD := int(lock.Fd()) //nolint:gosec // Unix file descriptors fit in int on every supported target.
	if err := syscall.Flock(lockFD, syscall.LOCK_EX); err != nil {
		return errors.WrapErr(err, "lock credential mutation")
	}
	defer func() { _ = syscall.Flock(lockFD, syscall.LOCK_UN) }()
	return operation()
}

func readCredentialState(path string) (credentialState, error) {
	state := credentialState{Version: 1, Servers: make(map[string]credentialMode)}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, errors.WrapErr(err, "read credential state")
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, errors.WrapErr(err, "decode credential state")
	}
	if state.Version != 1 {
		return state, fmt.Errorf("unsupported credential state version %d", state.Version)
	}
	if state.Servers == nil {
		state.Servers = make(map[string]credentialMode)
	}
	return state, nil
}

func loadCredentialMode(server string) (credentialMode, bool, error) {
	path, err := credentialStatePath()
	if err != nil {
		return credentialMode{}, false, err
	}
	state, err := readCredentialState(path)
	if err != nil {
		return credentialMode{}, false, err
	}
	mode, ok := state.Servers[server]
	if !ok {
		return credentialMode{}, false, nil
	}
	if !validCredentialSource(mode.Access, true) || !validCredentialSource(mode.Refresh, false) {
		return credentialMode{}, false, fmt.Errorf("invalid credential mode for %s", server)
	}
	return mode, true, nil
}

func validCredentialSource(source string, allowFile bool) bool {
	return source == credentialSourceNone || source == credentialSourceKeyring || (allowFile && source == credentialSourceFile)
}

func saveCredentialMode(server string, mode credentialMode) error {
	if server == "" || !validCredentialSource(mode.Access, true) || !validCredentialSource(mode.Refresh, false) {
		return errors.NewErr("invalid credential mode")
	}
	path, err := credentialStatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return errors.WrapErr(err, "create credential state directory")
	}

	state, err := readCredentialState(path)
	if err != nil {
		return err
	}
	state.Servers[server] = mode
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.WrapErr(err, "encode credential state")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".credential-state-*")
	if err != nil {
		return errors.WrapErr(err, "create credential state temp file")
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return errors.WrapErr(err, "secure credential state temp file")
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return errors.WrapErr(err, "write credential state")
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return errors.WrapErr(err, "sync credential state")
	}
	if err := tmp.Close(); err != nil {
		return errors.WrapErr(err, "close credential state")
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return errors.WrapErr(err, "replace credential state")
	}
	return syncDirectory(filepath.Dir(path))
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return errors.WrapErr(err, "open data directory for sync")
	}
	defer dir.Close()
	return errors.WrapErr(dir.Sync(), "sync data directory")
}
