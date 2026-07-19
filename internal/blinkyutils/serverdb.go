// Package blinkyutils preserves the released Blinky server-registry format.
package blinkyutils

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

const dataDirectoryName = "blinky-cli"

type StoredEndpoint struct {
	Username    string
	AccessToken string
}

type Registry struct {
	Default   string
	Endpoints map[string]StoredEndpoint
}

type serverWire struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registryWire struct {
	Default string                `json:"default"`
	Servers map[string]serverWire `json:"servers"`
}

func NewRegistry() Registry {
	return Registry{Endpoints: make(map[string]StoredEndpoint)}
}

func DataDirectory() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.WrapErr(err, "locate Blinky data directory")
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, dataDirectoryName), nil
}

func RegistryPath() (string, error) {
	dir, err := DataDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "servers.json"), nil
}

func DecodeRegistry(raw []byte) (Registry, error) {
	var wire registryWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Registry{}, errors.WrapErr(err, "decode Blinky server database")
	}
	registry := NewRegistry()
	registry.Default = wire.Default
	for name, server := range wire.Servers {
		registry.Endpoints[name] = StoredEndpoint{
			Username:    server.Username,
			AccessToken: server.Password,
		}
	}
	return registry, nil
}

func EncodeRegistry(registry Registry) ([]byte, error) {
	wire := registryWire{
		Default: registry.Default,
		Servers: make(map[string]serverWire, len(registry.Endpoints)),
	}
	for name, endpoint := range registry.Endpoints {
		wire.Servers[name] = serverWire{
			Username: endpoint.Username,
			Password: endpoint.AccessToken,
		}
	}
	raw, err := json.MarshalIndent(wire, "", "    ")
	if err != nil {
		return nil, errors.WrapErr(err, "encode Blinky server database")
	}
	return raw, nil
}
