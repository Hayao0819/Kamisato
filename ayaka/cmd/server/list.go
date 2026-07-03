package servercmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/spf13/cobra"
)

type serverRow struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Default  bool   `json:"default"`
	Secret   string `json:"secret,omitempty"`
}

const serverListDefaultFmt = `{{if .Default}}* {{else}}  {{end}}{{.Name}}{{if .Username}} ({{.Username}}{{if .Secret}}:{{.Secret}}{{end}}){{end}}`

func ListCmd() *cobra.Command {
	var (
		showSecret bool
		search     string
	)

	cmd := &cobra.Command{
		Use:   "list [<server>...]",
		Short: "List registered ayato servers",
		Args:  cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return blinkyutils.ServerNames(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := blinkyutils.ReadServerDB()
			if err != nil {
				return err
			}

			serverNames := make([]string, 0, len(db.Servers))
			for name := range db.Servers {
				if search == "" || strings.Contains(name, search) {
					serverNames = append(serverNames, name)
				}
			}
			sort.Strings(serverNames)
			if len(args) > 0 {
				serverNames = args
			}

			rows := make([]serverRow, 0, len(serverNames))
			for _, name := range serverNames {
				server, ok := db.Servers[name]
				if !ok {
					fmt.Fprintf(cmd.ErrOrStderr(), "server not found: %s\n", name)
					continue
				}
				row := serverRow{
					Name:     name,
					Username: server.Username,
					Default:  db.DefaultServer == name,
				}
				if showSecret {
					row.Secret = blinkyutils.LoadSecret(name, server.Password)
				}
				rows = append(rows, row)
			}

			format, err := cliutil.ResolveFormat(cmd, serverListDefaultFmt)
			if err != nil {
				return err
			}
			return cliutil.RenderList(cmd.OutOrStdout(), format, serverRow{}, rows)
		},
	}

	cmd.Flags().BoolVar(&showSecret, "show-secret", false, "Show stored password in output")
	cmd.Flags().StringVar(&search, "search", "", "Filter servers by name substring")
	cliutil.AddFormatFlags(cmd)

	return cmd
}
