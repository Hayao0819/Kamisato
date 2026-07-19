package servercmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/serverstore"
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
			return serverstore.Names(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoints, err := serverstore.ListEndpoints()
			if err != nil {
				return err
			}

			byName := make(map[string]serverstore.EndpointSummary, len(endpoints))
			serverNames := make([]string, 0, len(endpoints))
			for _, endpoint := range endpoints {
				byName[endpoint.URL] = endpoint
				if search == "" || strings.Contains(endpoint.URL, search) {
					serverNames = append(serverNames, endpoint.URL)
				}
			}
			if len(args) > 0 {
				serverNames = args
			}

			rows := make([]serverRow, 0, len(serverNames))
			for _, name := range serverNames {
				endpoint, ok := byName[name]
				if !ok {
					fmt.Fprintf(cmd.ErrOrStderr(), "server not found: %s\n", name)
					continue
				}
				row := serverRow{
					Name:     name,
					Username: endpoint.Username,
					Default:  endpoint.Default,
				}
				if showSecret {
					resolved, err := serverstore.Resolve(name)
					if err != nil {
						return err
					}
					row.Secret = resolved.AccessToken
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

	cmd.Flags().BoolVar(&showSecret, "show-secret", false, "Show the stored Bearer access token")
	cmd.Flags().StringVar(&search, "search", "", "Filter servers by name substring")
	cliutil.AddFormatFlags(cmd)

	return cmd
}
