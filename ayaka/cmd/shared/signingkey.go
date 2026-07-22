package shared

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

const (
	flagKeyHome        = "key-home"
	flagPassphraseFile = "passphrase-file"
	// PassphraseEnv is the env var holding the signing key passphrase, shared with
	// the miko local-sign path.
	PassphraseEnv = "AYAKA_SIGN_PASSPHRASE" // #nosec G101 -- environment variable name, not a credential
)

// AddKeyFlags registers the persistent flags every key/keyring command shares:
// where the signing key lives and how its passphrase is supplied.
func AddKeyFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String(flagKeyHome, "", "Signing key directory (default: <config>/kamisato/keys)")
	cmd.PersistentFlags().String(flagPassphraseFile, "", "File holding the key passphrase; env "+PassphraseEnv+" takes precedence")
}

// KeyDir resolves the signing key directory: --key-home when set, else
// <user-config-dir>/kamisato/keys.
func KeyDir(cmd *cobra.Command) (string, error) {
	if home, _ := cmd.Flags().GetString(flagKeyHome); home != "" {
		return home, nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", errors.WrapErr(err, "resolve config dir")
	}
	return filepath.Join(cfg, "kamisato", "keys"), nil
}

// Passphrase reads the key passphrase by precedence: env, then --passphrase-file.
// When neither is set and prompt is true and stdin is a terminal, it asks. An
// empty result means an unprotected key.
func Passphrase(cmd *cobra.Command, prompt bool) (string, error) {
	file, _ := cmd.Flags().GetString(flagPassphraseFile)
	var ask func() (string, error)
	if prompt && term.IsTerminal(int(syscall.Stdin)) {
		ask = func() (string, error) { return cliutil.PromptPassword("Key passphrase (empty for none):") }
	}
	return cliutil.ResolveSecret(PassphraseEnv, file, ask)
}

// LoadSigningKey opens the signing key, resolving the passphrase from env/file
// and, if that fails to decrypt and stdin is a terminal, prompting once. It
// returns the passphrase that unlocked the key so a command that re-saves the key
// (add/revoke/rotate a subkey) can re-encrypt it with the same passphrase rather
// than silently dropping the protection.
func LoadSigningKey(cmd *cobra.Command) (*sign.SigningKey, string, error) {
	dir, err := KeyDir(cmd)
	if err != nil {
		return nil, "", err
	}
	pass, err := Passphrase(cmd, false)
	if err != nil {
		return nil, "", err
	}
	k, err := sign.LoadSigningKey(dir, pass)
	if err == nil {
		return k, pass, nil
	}
	if pass == "" && term.IsTerminal(int(syscall.Stdin)) {
		prompted, perr := cliutil.PromptPassword("Key passphrase:")
		if perr != nil {
			return nil, "", perr
		}
		k, err := sign.LoadSigningKey(dir, prompted)
		if err != nil {
			return nil, "", err
		}
		return k, prompted, nil
	}
	return nil, "", err
}
