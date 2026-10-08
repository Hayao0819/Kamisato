package listcmd

import (
	"github.com/spf13/cobra"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/output"
	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/settings"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

const trustListDefaultFmt = "table {{.Kind}}\t{{.Name}}\t{{.Source}}\t{{.Maintainer}}\t{{.Commit}}"

type trustRow struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Source     string `json:"source,omitempty"`
	Maintainer string `json:"maintainer,omitempty"`
	Commit     string `json:"commit,omitempty"`
}

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List trusted maintainers and approved packages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := settings.Load(cmd.Flags())
			if err != nil {
				return err
			}
			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}

			format, err := cmdline.ResolveFormat(cmd, trustListDefaultFmt)
			if err != nil {
				return err
			}

			var rows []trustRow
			for _, m := range store.Maintainers() {
				rows = append(rows, trustRow{Kind: "maintainer", Name: m.Account, Source: m.Source})
			}
			for _, a := range store.Approvals() {
				rows = append(rows, trustRow{Kind: "package", Name: a.Pkgbase, Source: a.Source, Maintainer: a.Maintainer, Commit: output.Short(a.Commit)})
			}
			for _, w := range store.WhitelistEntries() {
				rows = append(rows, trustRow{Kind: "whitelist", Name: w.Pkgbase})
			}

			header := trustRow{Kind: "KIND", Name: "NAME", Source: "SOURCE", Maintainer: "MAINTAINER", Commit: "COMMIT"}
			return cmdline.RenderList(cmd.OutOrStdout(), format, header, rows)
		},
	}
	cmdline.AddFormatFlags(cmd)
	return cmd
}
