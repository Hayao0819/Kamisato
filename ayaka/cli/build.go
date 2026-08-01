package cli

import (
	"fmt"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/internal/buildset"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
)

type DirectBuildFlags struct {
	Arch         string
	LocalSources []string
	PacmanConf   string
	Backend      string
	Image        string
	Timeout      time.Duration
	WorkDir      string
	KeepWork     bool
}

func (o *DirectBuildFlags) Add(command *cobra.Command) {
	command.Flags().StringVar(&o.Arch, "arch", "x86_64", "Target pacman architecture")
	command.Flags().StringArrayVar(&o.LocalSources, "local-source", nil, "Local pkgbase directory; repeat for multiple sources")
	command.Flags().StringVar(&o.PacmanConf, "pacman-conf", "", "Pacman configuration used for resolution and builds")
	command.Flags().StringVar(&o.Backend, "backend", "", "Build backend (container)")
	command.Flags().StringVar(&o.Image, "image", "", "Container image; $arch is replaced with the target architecture")
	command.Flags().DurationVar(&o.Timeout, "timeout", 0, "Timeout per pkgbase")
	command.Flags().StringVar(&o.WorkDir, "work-dir", "", "Parent directory for temporary build state")
	command.Flags().BoolVar(&o.KeepWork, "keep-work", false, "Keep temporary build state after completion")
}

func (o *DirectBuildFlags) Validate(packages []string) error {
	if len(packages) == 0 && len(o.LocalSources) == 0 {
		return &cliutil.UsageError{Err: fmt.Errorf("pass at least one <pkgname> or --local-source")}
	}
	if o.PacmanConf == "" {
		return &cliutil.UsageError{Err: fmt.Errorf("--pacman-conf is required for direct package builds")}
	}
	return nil
}

func (o *DirectBuildFlags) PlanRequest(packages []string) buildset.PlanRequest {
	return buildset.PlanRequest{
		Arch:         o.Arch,
		Packages:     slices.Clone(packages),
		LocalSources: slices.Clone(o.LocalSources),
		PacmanConf:   o.PacmanConf,
		WorkDir:      o.WorkDir,
		KeepWork:     o.KeepWork,
	}
}

func (o *DirectBuildFlags) ApplicationOptions(configFile string) app.DirectBuildOptions {
	return app.DirectBuildOptions{
		ConfigFile: configFile,
		Arch:       o.Arch,
		Backend:    builder.Kind(o.Backend),
		Image:      o.Image,
		Timeout:    o.Timeout,
	}
}
