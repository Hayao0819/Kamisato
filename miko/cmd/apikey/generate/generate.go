package generatecmd

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/filesystem/safefile"
	mikoconfig "github.com/Hayao0819/Kamisato/miko/config"
)

func Cmd() *cobra.Command {
	var scopes []string
	var principal string
	cmd := &cobra.Command{
		Use:   "generate <name>",
		Short: "Generate an API key and append it to the JSON config",
		Long: "Generate a named 256-bit service key, append it to auth.api_keys with explicit scopes " +
			"(JSON config only), and print it once. --principal keeps job ownership stable while the unique key name changes during rotation. " +
			"The target is the inherited --config flag, or miko_config.json.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := generateAPIKey()
			if err != nil {
				return err
			}

			path, _ := cmd.Flags().GetString("config")
			if path == "" {
				path = "miko_config.json"
			}
			entry := mikoconfig.MikoAPIKey{Name: args[0], Principal: principal, Key: key, Scopes: scopes}
			if err := appendAPIKey(path, entry); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), key)
			fmt.Fprintf(cmd.ErrOrStderr(),
				"Appended named key %q to %s (mode 0600) with scopes %v.\n", args[0], path, scopes)
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&scopes, "scope", []string{apikey.ScopeBuildAdmin}, "allowed scope(s): build:submit, build:read, build:cancel, build:admin, sign")
	cmd.Flags().StringVar(&principal, "principal", "", "stable owner id shared by rotated keys (default: key name)")
	return cmd
}

func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.WrapErr(err, "failed to read random bytes")
	}
	return "miko_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}

func appendAPIKey(path string, entry mikoconfig.MikoAPIKey) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return errors.WrapErr(err, "failed to create API key config directory")
	}
	lock, err := safefile.Lock(filepath.Join(dir, "."+filepath.Base(path)+".lock"), 0o600)
	if err != nil {
		return errors.WrapErr(err, "failed to lock API key config")
	}
	defer func() { _ = lock.Unlock() }()

	cfg := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if len(data) > 0 {
			if err := json.Unmarshal(data, &cfg); err != nil {
				return errors.WrapErr(err, fmt.Sprintf("failed to parse %s (JSON config expected)", path))
			}
		}
	} else if !os.IsNotExist(err) {
		return errors.WrapErr(err, fmt.Sprintf("failed to read %s", path))
	}
	if cfg == nil {
		return errors.New("API key config must be a JSON object")
	}

	auth, exists := cfg["auth"]
	authFields, valid := auth.(map[string]any)
	if !exists {
		authFields = make(map[string]any)
	} else if !valid || authFields == nil {
		return errors.New("auth must be a JSON object")
	}
	keys, exists := authFields["api_keys"]
	keyEntries, valid := keys.([]any)
	if exists && !valid {
		return errors.New("auth.api_keys must be a JSON array")
	}
	var current []mikoconfig.MikoAPIKey
	if exists {
		raw, err := json.Marshal(keyEntries)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &current); err != nil {
			return errors.WrapErr(err, "invalid auth.api_keys")
		}
	}
	if err := (mikoconfig.MikoAuthConfig{APIKeys: append(current, entry)}).Validate(); err != nil {
		return err
	}
	serialized := map[string]any{"name": entry.Name, "key": entry.Key, "scopes": entry.Scopes}
	if entry.Principal != "" {
		serialized["principal"] = entry.Principal
	}
	authFields["api_keys"] = append(keyEntries, serialized)
	cfg["auth"] = authFields

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return safefile.WriteFile(path, append(out, '\n'), 0o600)
}
