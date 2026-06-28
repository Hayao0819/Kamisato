package cmd

import (
	"fmt"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	ayatosrc "github.com/Hayao0819/Kamisato/sara/ayato"
	"github.com/spf13/cobra"
)

func ayatoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ayato",
		Short: "Inspect federated ayato sources and their pinned signing keys",
	}
	cmd.AddCommand(ayatoListCmd(), ayatoPinCmd())
	return cmd
}

// ayatoListCmd shows the configured ayato sources next to the keys sara has
// pinned (whether hard-pinned in config or learned via TOFU).
func ayatoListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured ayato sources and pinned keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pins, err := ayatosrc.OpenPinStore(cfg.AyatoPinStorePath())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "sources:")
			for _, a := range cfg.Ayato {
				fmt.Fprintf(out, "  %s\t%s\t[%s]\n", a.Name, a.URL, sourceMode(a))
			}
			fmt.Fprintln(out, "pins:")
			for _, p := range pins.Entries() {
				fmt.Fprintf(out, "  %s\tkey_id %s\tlast_issued %s\n", p.Name, keyOrDash(p.KeyID), watermark(p.LastIssued))
			}
			return nil
		},
	}
}

// ayatoPinCmd fetches the key a source currently advertises and prints it so the
// operator can paste it into config as a hard pin, upgrading from TOFU.
func ayatoPinCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pin <name>",
		Short: "Fetch a source's advertised signing key to pin in config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			var src *conf.AyatoSource
			for i := range cfg.Ayato {
				if cfg.Ayato[i].Name == args[0] {
					src = &cfg.Ayato[i]
					break
				}
			}
			if src == nil {
				return utils.NewErrf("no ayato source named %q in config", args[0])
			}

			pub, keyID, err := ayatosrc.FetchPubkey(cmd.Context(), src.URL)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "# verify this key_id out of band before trusting it: %s\n", keyID)
			fmt.Fprintf(out, "# then set it on the %q source in config:\n", src.Name)
			fmt.Fprintf(out, "pubkey = %q\n", pub)
			return nil
		},
	}
}

func sourceMode(a conf.AyatoSource) string {
	switch {
	case a.Insecure:
		return "insecure"
	case a.Delegated():
		return "delegate"
	case a.PubKey != "":
		return "pinned"
	case a.Tofu:
		return "tofu"
	default:
		return "review"
	}
}

func keyOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func watermark(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format(time.RFC3339)
}
