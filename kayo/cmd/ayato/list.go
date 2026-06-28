package ayatocmd

import (
	"fmt"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	ayatosrc "github.com/Hayao0819/Kamisato/kayo/ayato"
	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
	"github.com/spf13/cobra"
)

func ayatoListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured ayato sources and pinned keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := shared.LoadConfig(cmd)
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

func sourceMode(a conf.AyatoSource) string {
	switch {
	case a.Insecure:
		return "insecure"
	case a.Delegated():
		return "delegate"
	case a.PubKey != "":
		return "pinned"
	case a.TrustOnFirstUse:
		return "first-use"
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
