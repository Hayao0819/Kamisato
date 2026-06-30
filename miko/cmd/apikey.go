package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
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
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate an API key and append it to the JSON config",
		Long: "Generate a 256-bit random API key, append it to the config's api_keys " +
			"(JSON config only), and print it once. Set the same value on ayato as " +
			"miko.api_key. Existing keys are kept so old and new keys overlap during rotation. " +
			"The target file is the inherited --config flag, or miko_config.json.",
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := generateAPIKey()
			if err != nil {
				return err
			}

			path, _ := cmd.Flags().GetString("config")
			if path == "" {
				path = "miko_config.json"
			}
			if err := appendAPIKey(path, key); err != nil {
				return err
			}

			// show-once: the key goes to stdout (never logged); guidance to stderr.
			fmt.Fprintln(cmd.OutOrStdout(), key)
			fmt.Fprintf(cmd.ErrOrStderr(),
				"Appended to %s (mode 0600). Set the same value on ayato as miko.api_key.\n", path)
			return nil
		},
	}
}

func generateAPIKey() (string, error) {
	b := make([]byte, 32) // 256 bits
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return "miko_" + base64.RawURLEncoding.EncodeToString(b), nil
}

// appendAPIKey adds key to the JSON config's api_keys array, creating the file
// if absent. Written 0600 because it holds a secret.
func appendAPIKey(path, key string) error {
	cfg := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if len(data) > 0 {
			if err := json.Unmarshal(data, &cfg); err != nil {
				return fmt.Errorf("failed to parse %s (JSON config expected): %w", path, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	var keys []string
	if raw, ok := cfg["api_keys"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				keys = append(keys, s)
			}
		}
	}
	cfg["api_keys"] = append(keys, key)

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o600); err != nil {
		return err
	}
	// WriteFile honors the mode only when creating; force 0600 on an existing file too.
	return os.Chmod(path, 0o600)
}
