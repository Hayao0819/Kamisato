package servercmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/spf13/cobra"
)

func ListCmd() *cobra.Command {
	var (
		showSecret bool
		showRaw    bool
		format     string
		search     string
	)

	cmd := &cobra.Command{
		Use:   "list [server_name...]",
		Short: "List ayato servers",
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

			for _, name := range serverNames {
				server, ok := db.Servers[name]
				if !ok {
					fmt.Fprintf(os.Stderr, "server not found: %s\n", name)
					continue
				}
				prefix := "  "
				if db.DefaultServer == name {
					prefix = "* "
				}
				if showRaw {
					b, _ := json.MarshalIndent(server, "", "  ") //nolint:gosec // dumps the user's own saved server config on explicit --raw
					fmt.Printf("%s%s: %s\n", prefix, name, string(b))
				} else if format == "json" {
					b, _ := json.Marshal(server) //nolint:gosec // dumps the user's own saved server config on explicit --format json
					fmt.Printf("%s%s: %s\n", prefix, name, string(b))
				} else {
					var line strings.Builder
					line.WriteString(prefix)
					line.WriteString(name)
					if server.Username != "" {
						line.WriteString(" (")
						line.WriteString(server.Username)
						if showSecret && server.Password != "" {
							line.WriteString(":" + server.Password)
						}
						line.WriteString(")")
					}
					fmt.Println(line.String())
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&showSecret, "show-secret", "s", false, "show server password")
	cmd.Flags().BoolVar(&showRaw, "raw", false, "show raw server config (json)")
	cmd.Flags().StringVar(&format, "format", "", "output format (json)")
	cmd.Flags().StringVar(&search, "search", "", "search server name (substring)")

	return cmd
}
