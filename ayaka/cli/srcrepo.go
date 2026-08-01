package cli

import (
	"strings"

	"github.com/samber/lo"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/internal/pacman/source"
)

// CompleteSrcRepoNames completes the first argument with the configured source
// repository names.
func CompleteSrcRepoNames(runtime *app.Runtime) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		a, err := runtime.App()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return a.GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
	}
}

func CompleteSrcRepoFlag(runtime *app.Runtime) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		a, err := runtime.App()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return a.GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
	}
}

func CompleteSrcRepoPackages(runtime *app.Runtime, selected func() string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		name := selected()
		if name == "" {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeSrcRepoPackageNames(runtime, name), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteSrcRepoThenPackages completes the first argument with source repo
// names and later arguments with that repo's package names.
func CompleteSrcRepoThenPackages(runtime *app.Runtime) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		a, err := runtime.App()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if len(args) == 0 {
			return a.GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
		}
		return sourceRepoPackageNames(a, args[0]), cobra.ShellCompDirectiveNoFileComp
	}
}

func completeSrcRepoPackageNames(runtime *app.Runtime, name string) []string {
	a, err := runtime.App()
	if err != nil {
		return nil
	}
	return sourceRepoPackageNames(a, name)
}

func sourceRepoPackageNames(a *app.App, name string) []string {
	sr := a.GetSrcRepo(name)
	if sr == nil {
		return nil
	}
	var candidates []string
	for _, sourcePackage := range sr.Pkgs {
		candidates = append(candidates, sourcePackage.Base())
		candidates = append(candidates, sourcePackage.Names()...)
	}
	return lo.Uniq(candidates)
}

// ResolveDiffServer picks the remote repo db dir: the explicit --diff-url, else
// the deprecated --server, else the arch-less repo.json url with the arch
// appended. Empty when none is configured.
func ResolveDiffServer(diffURL, server, configURL, arch string) string {
	if diffURL != "" {
		return diffURL
	}
	if server != "" {
		return server
	}
	if configURL != "" {
		return strings.TrimRight(configURL, "/") + "/" + arch
	}
	return ""
}

// RemoteRepo fetches the published repo db per ResolveDiffServer; a repo/arch
// with no db yet resolves to an empty repo, so first runs plan/build everything.
func RemoteRepo(diffURL, server string, src *source.SourceRepo, arch string) (*repo.RemoteRepo, error) {
	dburl := ResolveDiffServer(diffURL, server, src.Config.URL, arch)
	if dburl == "" {
		return nil, errors.NewErr("source repo " + src.Config.Name + " has no url in repo.json; pass --diff-url")
	}
	rr, err := repo.FetchOrEmpty(dburl, src.Config.Name)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to read remote repo db")
	}
	return rr, nil
}
