package raiou

import (
	"io"

	go_srcinfo "github.com/Morganamilo/go-srcinfo"
)

// ArchStrings はアーキテクチャごとの値（depends, sums など）を保持します。
// 同一 Arch に複数値があり得るため、キーごとにスライスで保持します。
type ArchStrings map[string][]string

// SrcinfoPackage は .SRCINFO 内の 1 パッケージ定義を表します。
type SrcinfoPackage struct {
	PkgName    string      `mapstructure:"pkgname" json:"pkgname" yml:"pkgname" toml:"pkgname"`
	PkgDesc    string      `mapstructure:"pkgdesc" json:"pkgdesc" yml:"pkgdesc" toml:"pkgdesc"`
	PkgArch    []string    `mapstructure:"pkgarch" json:"pkgarch" yml:"pkgarch" toml:"pkgarch"`
	URL        string      `mapstructure:"url" json:"url" yml:"url" toml:"url"`
	License    []string    `mapstructure:"license" json:"license" yml:"license" toml:"license"`
	Groups     []string    `mapstructure:"groups" json:"groups" yml:"groups" toml:"groups"`
	Depends    ArchStrings `mapstructure:"depends" json:"depends" yml:"depends" toml:"depends"`
	OptDepends ArchStrings `mapstructure:"optdepends" json:"optdepends" yml:"optdepends" toml:"optdepends"`
	Provides   ArchStrings `mapstructure:"provides" json:"provides" yml:"provides" toml:"provides"`
	Conflicts  ArchStrings `mapstructure:"conflicts" json:"conflicts" yml:"conflicts" toml:"conflicts"`
	Replaces   ArchStrings `mapstructure:"replaces" json:"replaces" yml:"replaces" toml:"replaces"`
	Backup     []string    `mapstructure:"backup" json:"backup" yml:"backup" toml:"backup"`
	Options    []string    `mapstructure:"options" json:"options" yml:"options" toml:"options"`
	Install    string      `mapstructure:"install" json:"install" yml:"install" toml:"install"`
	Changelog  string      `mapstructure:"changelog" json:"changelog" yml:"changelog" toml:"changelog"`
}

// SrcinfoBase は .SRCINFO の pkgbase（全パッケージ共通のビルド情報）を表します。
type SrcinfoBase struct {
	PkgBase      string      `mapstructure:"pkgbase" json:"pkgbase" yml:"pkgbase" toml:"pkgbase"`
	PkgVer       string      `mapstructure:"pkgver" json:"pkgver" yml:"pkgver" toml:"pkgver"`
	PkgRel       string      `mapstructure:"pkgrel" json:"pkgrel" yml:"pkgrel" toml:"pkgrel"`
	PkgEpoch     string      `mapstructure:"epoch" json:"epoch" yml:"epoch" toml:"epoch"`
	Source       ArchStrings `mapstructure:"source" json:"source" yml:"source" toml:"source"`
	ValidPGPKeys []string    `mapstructure:"validpgpkeys" json:"validpgpkeys" yml:"validpgpkeys" toml:"validpgpkeys"`
	NoExtract    []string    `mapstructure:"noextract" json:"noextract" yml:"noextract" toml:"noextract"`
	MD5Sums      ArchStrings `mapstructure:"md5sums" json:"md5sums" yml:"md5sums" toml:"md5sums"`
	SHA1Sums     ArchStrings `mapstructure:"sha1sums" json:"sha1sums" yml:"sha1sums" toml:"sha1sums"`
	SHA224Sums   ArchStrings `mapstructure:"sha224sums" json:"sha224sums" yml:"sha224sums" toml:"sha224sums"`
	SHA256Sums   ArchStrings `mapstructure:"sha256sums" json:"sha256sums" yml:"sha256sums" toml:"sha256sums"`
	SHA384Sums   ArchStrings `mapstructure:"sha384sums" json:"sha384sums" yml:"sha384sums" toml:"sha384sums"`
	SHA512Sums   ArchStrings `mapstructure:"sha512sums" json:"sha512sums" yml:"sha512sums" toml:"sha512sums"`
	B2Sums       ArchStrings `mapstructure:"b2sums" json:"b2sums" yml:"b2sums" toml:"b2sums"`
	MakeDepends  ArchStrings `mapstructure:"makedepends" json:"makedepends" yml:"makedepends" toml:"makedepends"`
	CheckDepends ArchStrings `mapstructure:"checkdepends" json:"checkdepends" yml:"checkdepends" toml:"checkdepends"`
}

// SRCINFO は .SRCINFO ファイル全体（pkgbase + 各パッケージ）を表します。
type SRCINFO struct {
	SrcinfoBase    `mapstructure:",squash"`
	SrcinfoPackage `mapstructure:",squash"`
	Packages       []SrcinfoPackage `mapstructure:"packages" json:"packages" yml:"packages" toml:"packages"`
}

// ParseSrcinfo は io.Reader から .SRCINFO を読み取ります。
func ParseSrcinfo(r io.Reader) (*SRCINFO, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	parsed, err := go_srcinfo.Parse(string(b))
	if err != nil {
		return nil, err
	}
	return srcinfoFromGo(parsed), nil
}

func ParseSrcinfoFile(path string) (*SRCINFO, error) {
	parsed, err := go_srcinfo.ParseFile(path)
	if err != nil {
		return nil, err
	}
	return srcinfoFromGo(parsed), nil
}

func ParseSrcinfoString(data string) (*SRCINFO, error) {
	parsed, err := go_srcinfo.Parse(data)
	if err != nil {
		return nil, err
	}
	return srcinfoFromGo(parsed), nil
}

// srcinfoPackageFromGo converts a go_srcinfo.Package to a SrcinfoPackage.
func srcinfoPackageFromGo(pkg *go_srcinfo.Package) *SrcinfoPackage {
	if pkg == nil {
		return nil
	}

	out := &SrcinfoPackage{
		PkgName:   pkg.Pkgname,
		PkgDesc:   pkg.Pkgdesc,
		PkgArch:   pkg.Arch,
		URL:       pkg.URL,
		License:   pkg.License,
		Groups:    pkg.Groups,
		Backup:    pkg.Backup,
		Options:   pkg.Options,
		Install:   pkg.Install,
		Changelog: pkg.Changelog,
	}

	out.Depends = archStringsFromGo(pkg.Depends)
	out.OptDepends = archStringsFromGo(pkg.OptDepends)
	out.Provides = archStringsFromGo(pkg.Provides)
	out.Conflicts = archStringsFromGo(pkg.Conflicts)
	out.Replaces = archStringsFromGo(pkg.Replaces)

	return out
}

// srcinfoBaseFromGo converts a go_srcinfo.PackageBase to a SrcinfoBase.
func srcinfoBaseFromGo(pkgBase *go_srcinfo.PackageBase) *SrcinfoBase {
	if pkgBase == nil {
		return nil
	}

	out := &SrcinfoBase{
		PkgBase:      pkgBase.Pkgbase,
		PkgVer:       pkgBase.Pkgver,
		PkgRel:       pkgBase.Pkgrel,
		PkgEpoch:     pkgBase.Epoch,
		ValidPGPKeys: pkgBase.ValidPGPKeys,
		NoExtract:    pkgBase.NoExtract,
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

// srcinfoFromGo converts a go_srcinfo.Srcinfo to a SRCINFO.
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

// archStringsFromGo converts a slice of go_srcinfo.ArchString to ArchStrings.
func archStringsFromGo(archStrings []go_srcinfo.ArchString) ArchStrings {
	if archStrings == nil {
		return nil
	}
	result := make(ArchStrings)
	for _, archString := range archStrings {
		result[archString.Arch] = append(result[archString.Arch], archString.Value)
	}

	return result
}
