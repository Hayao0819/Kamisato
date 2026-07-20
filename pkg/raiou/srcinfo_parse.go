package raiou

import (
	"fmt"
	"os"
	"strings"

	go_srcinfo "github.com/Morganamilo/go-srcinfo"
)

func ParseSrcinfoFile(path string) (*SRCINFO, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read .SRCINFO %q: %w", path, err)
	}
	return ParseSrcinfoString(string(data))
}

func ParseSrcinfoString(data string) (*SRCINFO, error) {
	parsed, err := go_srcinfo.Parse(data)
	if err != nil {
		return nil, err
	}
	out := srcinfoFromGo(parsed)
	// go-srcinfo v1.0.0 predates cksums support and deliberately ignores
	// unknown keys. Recover this current makepkg field without forking its
	// parser, while retaining its section and architecture validation.
	out.CKSums = srcinfoArchField(data, "cksums")
	return out, nil
}

func srcinfoPackageFromGo(pkg *go_srcinfo.Package) *SrcinfoPackage {
	if pkg == nil {
		return nil
	}

	out := &SrcinfoPackage{PkgName: pkg.Pkgname}
	out.PkgDesc, out.overrides = scalarFromGo(pkg.Pkgdesc, overridePkgDesc, out.overrides)
	out.URL, out.overrides = scalarFromGo(pkg.URL, overrideURL, out.overrides)
	out.Install, out.overrides = scalarFromGo(pkg.Install, overrideInstall, out.overrides)
	out.Changelog, out.overrides = scalarFromGo(pkg.Changelog, overrideChangelog, out.overrides)
	out.PkgArch = stringsFromGo(pkg.Arch)
	out.License = stringsFromGo(pkg.License)
	out.Groups = stringsFromGo(pkg.Groups)
	out.Backup = stringsFromGo(pkg.Backup)
	out.Options = stringsFromGo(pkg.Options)
	out.Depends = archStringsFromGo(pkg.Depends)
	out.OptDepends = archStringsFromGo(pkg.OptDepends)
	out.Provides = archStringsFromGo(pkg.Provides)
	out.Conflicts = archStringsFromGo(pkg.Conflicts)
	out.Replaces = archStringsFromGo(pkg.Replaces)
	return out
}

func srcinfoBaseFromGo(pkgBase *go_srcinfo.PackageBase) *SrcinfoBase {
	if pkgBase == nil {
		return nil
	}

	out := &SrcinfoBase{
		PkgBase:      pkgBase.Pkgbase,
		PkgVer:       normalizeGoValue(pkgBase.Pkgver),
		PkgRel:       normalizeGoValue(pkgBase.Pkgrel),
		PkgEpoch:     normalizeGoValue(pkgBase.Epoch),
		ValidPGPKeys: stringsFromGo(pkgBase.ValidPGPKeys),
		NoExtract:    stringsFromGo(pkgBase.NoExtract),
	}
	out.Source = archStringsFromGo(pkgBase.Source)
	out.MD5Sums = archStringsFromGo(pkgBase.MD5Sums)
	out.SHA1Sums = archStringsFromGo(pkgBase.SHA1Sums)
	out.SHA224Sums = archStringsFromGo(pkgBase.SHA224Sums)
	out.SHA256Sums = archStringsFromGo(pkgBase.SHA256Sums)
	out.SHA384Sums = archStringsFromGo(pkgBase.SHA384Sums)
	out.SHA512Sums = archStringsFromGo(pkgBase.SHA512Sums)
	out.B2Sums = archStringsFromGo(pkgBase.B2Sums)
	out.MakeDepends = archStringsFromGo(pkgBase.MakeDepends)
	out.CheckDepends = archStringsFromGo(pkgBase.CheckDepends)
	return out
}

func srcinfoFromGo(srcinfo *go_srcinfo.Srcinfo) *SRCINFO {
	if srcinfo == nil {
		return nil
	}

	out := &SRCINFO{
		SrcinfoBase:    *srcinfoBaseFromGo(&srcinfo.PackageBase),
		SrcinfoPackage: *srcinfoPackageFromGo(&srcinfo.Package),
		Packages:       make([]SrcinfoPackage, len(srcinfo.Packages)),
	}
	for i, pkg := range srcinfo.Packages {
		out.Packages[i] = *srcinfoPackageFromGo(&pkg)
	}
	return out
}

func archStringsFromGo(archStrings []go_srcinfo.ArchString) ArchStrings {
	if archStrings == nil {
		return nil
	}
	result := make(ArchStrings)
	for _, value := range archStrings {
		result[value.Arch] = append(result[value.Arch], normalizeGoValue(value.Value))
	}
	return result
}

func scalarFromGo(value string, field, overrides scalarOverrides) (string, scalarOverrides) {
	if value == go_srcinfo.EmptyOverride {
		return "", overrides | field
	}
	return value, overrides
}

func normalizeGoValue(value string) string {
	if value == go_srcinfo.EmptyOverride {
		return ""
	}
	return value
}

func stringsFromGo(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = normalizeGoValue(value)
	}
	return out
}

func srcinfoArchField(data, field string) ArchStrings {
	var out ArchStrings
	inBase := false
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue // go-srcinfo reports the malformed line first
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "pkgbase":
			inBase = true
			continue
		case "pkgname":
			inBase = false
			continue
		}
		if !inBase {
			continue
		}

		arch := ""
		switch {
		case key == field:
		case strings.HasPrefix(key, field+"_"):
			arch = strings.TrimPrefix(key, field+"_")
		default:
			continue
		}
		if out == nil {
			out = make(ArchStrings)
		}
		out[arch] = append(out[arch], value)
	}
	return out
}
