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
				if showRaw || format == "json" {
					// The password stays behind --show-secret in every format, so
					// piping list output to a file or a script never leaks it. The
					// live secret may live in the OS keyring, so resolve it there.
					out := server
					if showSecret {
						out.Password = blinkyutils.LoadSecret(name, server.Password)
					} else {
						out.Password = ""
					}
					var b []byte
					if showRaw {
						b, _ = json.MarshalIndent(out, "", "  ") //nolint:gosec // G117: Password is redacted above unless --show-secret is explicit
					} else {
						b, _ = json.Marshal(out) //nolint:gosec // G117: Password is redacted above unless --show-secret is explicit
					}
					fmt.Printf("%s%s: %s\n", prefix, name, string(b))
				} else {
					var line strings.Builder
					line.WriteString(prefix)
					line.WriteString(name)
					if server.Username != "" {
						line.WriteString(" (")
						line.WriteString(server.Username)
						if secret := blinkyutils.LoadSecret(name, server.Password); showSecret && secret != "" {
							line.WriteString(":" + secret)
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
