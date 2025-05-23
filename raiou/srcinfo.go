package raiou

type ArchStrings map[string]string

type Package struct {
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

type PackageBase struct {
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

type SRCINFO struct {
	PackageBase `mapstructure:",squash"`
	Package     `mapstructure:",squash"`
	Packages    []Package `mapstructure:"packages" json:"packages" yml:"packages" toml:"packages"`
}
