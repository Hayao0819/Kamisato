package raiou

import "fmt"

// SplitPackages resolves every pkgname section against the pkgbase-level
// package fields. Unlike go-srcinfo, it never returns the internal NUL marker
// used to represent explicit empty overrides.
func (s *SRCINFO) SplitPackages() []SrcinfoPackage {
	if s == nil {
		return nil
	}
	out := make([]SrcinfoPackage, len(s.Packages))
	for i := range s.Packages {
		out[i] = mergeSrcinfoPackage(s.SrcinfoPackage, s.Packages[i])
	}
	return out
}

// SplitPackage resolves one named pkgname section against pkgbase-level fields.
func (s *SRCINFO) SplitPackage(name string) (*SrcinfoPackage, error) {
	if s != nil {
		for i := range s.Packages {
			if s.Packages[i].PkgName == name {
				pkg := mergeSrcinfoPackage(s.SrcinfoPackage, s.Packages[i])
				return &pkg, nil
			}
		}
	}

	base := ""
	if s != nil {
		base = s.PkgBase
	}
	return nil, fmt.Errorf("package %q is not part of package base %q", name, base)
}

func mergeSrcinfoPackage(global, override SrcinfoPackage) SrcinfoPackage {
	out := cleanSrcinfoPackage(global)
	out.PkgName = override.PkgName

	overrideScalar(&out.PkgDesc, override.PkgDesc, override.overrides&overridePkgDesc != 0)
	overrideScalar(&out.URL, override.URL, override.overrides&overrideURL != 0)
	overrideScalar(&out.Install, override.Install, override.overrides&overrideInstall != 0)
	overrideScalar(&out.Changelog, override.Changelog, override.overrides&overrideChangelog != 0)

	out.PkgArch = mergeStringOverride(out.PkgArch, override.PkgArch)
	out.License = mergeStringOverride(out.License, override.License)
	out.Groups = mergeStringOverride(out.Groups, override.Groups)
	out.Backup = mergeStringOverride(out.Backup, override.Backup)
	out.Options = mergeStringOverride(out.Options, override.Options)
	out.Depends = mergeArchOverride(out.Depends, override.Depends)
	out.OptDepends = mergeArchOverride(out.OptDepends, override.OptDepends)
	out.Provides = mergeArchOverride(out.Provides, override.Provides)
	out.Conflicts = mergeArchOverride(out.Conflicts, override.Conflicts)
	out.Replaces = mergeArchOverride(out.Replaces, override.Replaces)
	out.overrides = 0
	return out
}

func cleanSrcinfoPackage(pkg SrcinfoPackage) SrcinfoPackage {
	pkg.PkgArch = appendNonEmpty(nil, pkg.PkgArch...)
	pkg.License = appendNonEmpty(nil, pkg.License...)
	pkg.Groups = appendNonEmpty(nil, pkg.Groups...)
	pkg.Backup = appendNonEmpty(nil, pkg.Backup...)
	pkg.Options = appendNonEmpty(nil, pkg.Options...)
	pkg.Depends = cleanArchStrings(pkg.Depends)
	pkg.OptDepends = cleanArchStrings(pkg.OptDepends)
	pkg.Provides = cleanArchStrings(pkg.Provides)
	pkg.Conflicts = cleanArchStrings(pkg.Conflicts)
	pkg.Replaces = cleanArchStrings(pkg.Replaces)
	pkg.overrides = 0
	return pkg
}

func overrideScalar(dst *string, value string, explicitlyEmpty bool) {
	if value != "" || explicitlyEmpty {
		*dst = value
	}
}

func mergeStringOverride(global, override []string) []string {
	if override == nil {
		return global
	}
	return appendNonEmpty(nil, override...)
}

func mergeArchOverride(global, override ArchStrings) ArchStrings {
	if override == nil {
		return global
	}

	out := make(ArchStrings, len(global)+len(override))
	for arch, values := range global {
		if _, overridden := override[arch]; overridden {
			continue
		}
		if clean := appendNonEmpty(nil, values...); len(clean) > 0 {
			out[arch] = clean
		}
	}
	for arch, values := range override {
		if clean := appendNonEmpty(nil, values...); len(clean) > 0 {
			out[arch] = clean
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cleanArchStrings(values ArchStrings) ArchStrings {
	if values == nil {
		return nil
	}
	out := make(ArchStrings, len(values))
	for arch, group := range values {
		if clean := appendNonEmpty(nil, group...); len(clean) > 0 {
			out[arch] = clean
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func appendNonEmpty(dst []string, values ...string) []string {
	for _, value := range values {
		if value != "" {
			dst = append(dst, value)
		}
	}
	return dst
}
