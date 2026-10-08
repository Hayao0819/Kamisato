package plancmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/buildenv"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/buildflags"
	"github.com/Hayao0819/Kamisato/ayaka/config"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var input buildflags.Options
	var jsonOutput bool
	command := &cobra.Command{
		Use:               "plan [pkgname...]",
		Short:             "Resolve package sources and print their build order",
		ValidArgsFunction: cobra.NoFileCompletions,
		PreRunE: func(_ *cobra.Command, args []string) error {
			return input.Validate(args)
		},
		RunE: func(command *cobra.Command, args []string) error {
			configFile, err := command.Flags().GetString("config")
			if err != nil {
				return err
			}
			host, err := config.LoadDirectBuildHostConfig(configFile)
			if err != nil {
				return err
			}
			service, _, err := buildenv.New(host, input.BackendOptions())
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			plan, err := service.Plan(ctx, input.PlanRequest(args))
			if err != nil {
				return err
			}
			if jsonOutput {
				encoder := json.NewEncoder(command.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(plan)
			}
			if len(plan.BuildOrder) == 0 {
				_, err = fmt.Fprintln(command.OutOrStdout(), "No packages to build.")
				return err
			}
			for index, pkgbase := range plan.BuildOrder {
				if _, err := fmt.Fprintf(command.OutOrStdout(), "%d\t%s\n", index+1, pkgbase); err != nil {
					return err
				}
			}
			return nil
		},
	}
	input.Add(command)
	command.Flags().BoolVar(&jsonOutput, "json", false, "Write the complete plan as JSON")
	return command
}
