package serverstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/atomicfile"
	"github.com/Hayao0819/Kamisato/pkg/filelock"
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
	dir, err := blinkyutils.DataDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credential-state.json"), nil
}

var credentialMutationMu sync.Mutex

// withCredentialMutation serializes credential state changes.
func withCredentialMutation(operation func() error) error {
	credentialMutationMu.Lock()
	defer credentialMutationMu.Unlock()
	dir, err := blinkyutils.DataDirectory()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.WrapErr(err, "create server data directory")
	}
	lock, err := filelock.Acquire(filepath.Join(dir, "credentials.lock"), 0o600)
	if err != nil {
		return errors.WrapErr(err, "lock credential mutation")
	}
	defer func() { _ = lock.Release() }()
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
	return errors.WrapErr(atomicfile.WriteFile(path, raw, 0o600), "save credential state")
}
