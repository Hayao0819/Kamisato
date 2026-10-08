package keyinput

import (
	"os"
	"path/filepath"
	"syscall"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/sign"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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

// Options is independent of Cobra; commands decode flags once before using it.
type Options struct {
	Home           string
	PassphraseFile string
}

// Provider makes key storage, secret input, and terminal interaction explicit.
// Options contains values only and can be reused without a Cobra command.
type Provider struct {
	Secret  func(string, string, func() (string, error)) (string, error)
	Prompt  func(string) (string, error)
	LoadKey func(string, string) (*sign.SigningKey, error)
}

func DefaultProvider() Provider {
	provider := Provider{
		Secret:  cmdline.ResolveSecret,
		LoadKey: sign.LoadSigningKey,
	}
	if term.IsTerminal(int(syscall.Stdin)) {
		provider.Prompt = cmdline.PromptPassword
	}
	return provider
}

func Read(cmd *cobra.Command) Options {
	home, _ := cmd.Flags().GetString(flagKeyHome)
	file, _ := cmd.Flags().GetString(flagPassphraseFile)
	return Options{Home: home, PassphraseFile: file}
}

// Directory resolves the signing key directory: --key-home when set, else
// <user-config-dir>/kamisato/keys.
func Directory(o Options) (string, error) {
	if o.Home != "" {
		return o.Home, nil
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
func (p Provider) Passphrase(o Options, prompt bool) (string, error) {
	var ask func() (string, error)
	if prompt && p.Prompt != nil {
		ask = func() (string, error) { return p.Prompt("Key passphrase (empty for none):") }
	}
	return p.Secret(PassphraseEnv, o.PassphraseFile, ask)
}

// Load opens the signing key, resolving the passphrase from env/file
// and, if that fails to decrypt and stdin is a terminal, prompting once. It
// returns the passphrase that unlocked the key so a command that re-saves the key
// (add/revoke/rotate a subkey) can re-encrypt it with the same passphrase rather
// than silently dropping the protection.
func (p Provider) Load(o Options) (*sign.SigningKey, string, error) {
	dir, err := Directory(o)
	if err != nil {
		return nil, "", err
	}
	pass, err := p.Passphrase(o, false)
	if err != nil {
		return nil, "", err
	}
	k, err := p.LoadKey(dir, pass)
	if err == nil {
		return k, pass, nil
	}
	if pass == "" && p.Prompt != nil {
		prompted, perr := p.Prompt("Key passphrase:")
		if perr != nil {
			return nil, "", perr
		}
		k, err := p.LoadKey(dir, prompted)
		if err != nil {
			return nil, "", err
		}
		return k, prompted, nil
	}
	return nil, "", err
}
