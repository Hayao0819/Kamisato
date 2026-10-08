package completion

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

// CompleteSrcRepoNames completes the first argument with the configured source
// repository names.
func CompleteSrcRepoNames(sources sourcerepos.Reader) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		repos, err := sources.All()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return repositoryNames(repos), cobra.ShellCompDirectiveNoFileComp
	}
}

func CompleteSrcRepoFlag(sources sourcerepos.Reader) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		repos, err := sources.All()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return repositoryNames(repos), cobra.ShellCompDirectiveNoFileComp
	}
}

func CompleteSrcRepoPackages(sources sourcerepos.Reader, selected func() string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		name := selected()
		if name == "" {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeSrcRepoPackageNames(sources, name), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteSrcRepoThenPackages completes the first argument with source repo
// names and later arguments with that repo's package names.
func CompleteSrcRepoThenPackages(sources sourcerepos.Reader) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			repos, err := sources.All()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return repositoryNames(repos), cobra.ShellCompDirectiveNoFileComp
		}
		return completeSrcRepoPackageNames(sources, args[0]), cobra.ShellCompDirectiveNoFileComp
	}
}

func completeSrcRepoPackageNames(sources sourcerepos.Reader, name string) []string {
	sr, err := sources.Find(name)
	if err != nil {
		return nil
	}
	return sourceRepoPackageNames(sr)
}

func repositoryNames(repos []*source.SourceRepo) []string {
	return lo.Map(repos, func(repo *source.SourceRepo, _ int) string { return repo.Config.Name })
}

func sourceRepoPackageNames(sr *source.SourceRepo) []string {
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
