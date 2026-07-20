package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/safefile"
)

func apikeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apikey",
		Short: "Manage miko API keys",
	}
	cmd.AddCommand(apikeyGenerateCmd())
	return cmd
}

func apikeyGenerateCmd() *cobra.Command {
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
			entry := conf.MikoAPIKey{Name: args[0], Principal: principal, Key: key, Scopes: scopes}
			if err := appendAPIKey(path, entry); err != nil {
				return err
			}

			// show-once: the key goes to stdout (never logged); guidance to stderr.
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
	b := make([]byte, 32) // 256 bits
	if _, err := rand.Read(b); err != nil {
		return "", errors.WrapErr(err, "failed to read random bytes")
	}
	return "miko_" + base64.RawURLEncoding.EncodeToString(b), nil
}

// appendAPIKey stores a named API key.
func appendAPIKey(path string, entry conf.MikoAPIKey) error {
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

	auth, _ := cfg["auth"].(map[string]any)
	if auth == nil {
		auth = make(map[string]any)
	}
	keys, _ := auth["api_keys"].([]any)
	serialized := map[string]any{"name": entry.Name, "key": entry.Key, "scopes": entry.Scopes}
	if entry.Principal != "" {
		serialized["principal"] = entry.Principal
	}
	auth["api_keys"] = append(keys, serialized)
	cfg["auth"] = auth

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return safefile.WriteFile(path, append(out, '\n'), 0o600)
}
