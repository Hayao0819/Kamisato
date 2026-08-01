package plancmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cli"
	"github.com/spf13/cobra"
)

type options struct {
	input cli.DirectBuildFlags
	json  bool
}

func Cmd() *cobra.Command {
	var options options
	command := &cobra.Command{
		Use:               "plan [pkgname...]",
		Short:             "Resolve package sources and print their build order",
		ValidArgsFunction: cobra.NoFileCompletions,
		PreRunE: func(_ *cobra.Command, args []string) error {
			return options.input.Validate(args)
		},
		RunE: func(command *cobra.Command, args []string) error {
			configFile, err := command.Flags().GetString("config")
			if err != nil {
				return err
			}
			application, _, err := app.NewDirectBuildApplication(options.input.ApplicationOptions(configFile))
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			plan, err := application.Plan(ctx, options.input.PlanRequest(args))
			if err != nil {
				return err
			}
			if options.json {
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
	options.input.Add(command)
	command.Flags().BoolVar(&options.json, "json", false, "Write the complete plan as JSON")
	return command
}
