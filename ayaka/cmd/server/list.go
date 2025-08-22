// No changes needed: already in English
package servercmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	blinky_utils "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

func ListCmd() *cobra.Command {
	var (
		showSecret bool
		showRaw    bool
		format     string
		copyField  string
		search     string
	)

	cmd := &cobra.Command{
		Use:   "list [server_name...]",
		Short: "List Blinky servers",
		Args:  cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			db, err := blinky_utils.ReadServerDB()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var completions []string
			for name := range db.Servers {
				if strings.HasPrefix(name, toComplete) {
					completions = append(completions, name)
				}
			}
			sort.Strings(completions)
			return completions, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := blinky_utils.ReadServerDB()
			if err != nil {
				return utils.WrapErr(err, "failed to read server database")
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
					b, _ := json.MarshalIndent(server, "", "  ")
					fmt.Printf("%s%s: %s\n", prefix, name, string(b))
				} else if format == "json" {
					b, _ := json.Marshal(server)
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
				if copyField != "" {
					var val string
					switch copyField {
					case "username":
						val = server.Username
					case "password":
						val = server.Password
					case "url":
						val = name
					}
					if val != "" {
						_ = clipboard.WriteAll(val)
						fmt.Printf("Copied %s to clipboard\n", copyField)
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&showSecret, "show-secret", "s", false, "show server password")
	cmd.Flags().BoolVar(&showRaw, "raw", false, "show raw server config (json)")
	cmd.Flags().StringVar(&format, "format", "", "output format (json)")
	cmd.Flags().StringVar(&copyField, "copy", "", "copy field to clipboard (username|password|url)")
	cmd.Flags().StringVar(&search, "search", "", "search server name (substring)")

	return cmd
}
