package shared

import (
	"strings"

	"github.com/samber/lo"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// CompleteSrcRepoNames completes the first argument with the configured source
// repository names.
func CompleteSrcRepoNames(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return app.From(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// CompleteSrcRepoThenPackages completes the first argument with source repo
// names and later arguments with that repo's package names.
func CompleteSrcRepoThenPackages(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	a := app.From(cmd)
	if len(args) == 0 {
		return a.GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
	}
	sr := a.GetSrcRepo(args[0])
	if sr == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var cands []string
	for _, p := range sr.Pkgs {
		cands = append(cands, p.Base())
		cands = append(cands, p.Names()...)
	}
	return lo.Uniq(cands), cobra.ShellCompDirectiveNoFileComp
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
func RemoteRepo(diffURL, server string, src *repo.SourceRepo, arch string) (*repo.RemoteRepo, error) {
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
