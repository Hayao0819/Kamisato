package ayatocmd

import (
	"time"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/kayo/ayatosrc"
	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
	"github.com/spf13/cobra"
)

const ayatoListDefaultFmt = "table {{.Kind}}\t{{.Name}}\t{{.URL}}\t{{.Mode}}\t{{.KeyID}}\t{{.LastIssued}}"

type ayatoRow struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	URL        string `json:"url,omitempty"`
	Mode       string `json:"mode,omitempty"`
	KeyID      string `json:"key_id,omitempty"`
	LastIssued string `json:"last_issued,omitempty"`
}

func ayatoListCmd() *cobra.Command {
	cmd := &cobra.Command{
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

			format, err := cliutil.ResolveFormat(cmd, ayatoListDefaultFmt)
			if err != nil {
				return err
			}

			var rows []ayatoRow
			for _, a := range cfg.Ayato {
				rows = append(rows, ayatoRow{Kind: "source", Name: a.Name, URL: a.URL, Mode: sourceMode(a)})
			}
			for _, p := range pins.Entries() {
				rows = append(rows, ayatoRow{Kind: "pin", Name: p.Name, KeyID: keyOrDash(p.KeyID), LastIssued: watermark(p.LastIssued)})
			}

			header := ayatoRow{Kind: "KIND", Name: "NAME", URL: "URL", Mode: "MODE", KeyID: "KEY_ID", LastIssued: "LAST_ISSUED"}
			return cliutil.RenderList(cmd.OutOrStdout(), format, header, rows)
		},
	}
	cliutil.AddFormatFlags(cmd)
	return cmd
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
