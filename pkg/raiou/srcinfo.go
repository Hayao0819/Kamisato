package raiou

// ArchStrings maps an architecture to its values; a single arch can have multiple.
type ArchStrings map[string][]string

// ForArch returns architecture-independent values followed by values specific
// to arch. Empty override markers are omitted.
func (a ArchStrings) ForArch(arch string) []string {
	out := appendNonEmpty(nil, a[""]...)
	if arch != "" {
		out = appendNonEmpty(out, a[arch]...)
	}
	return out
}

// All returns every non-empty value. The order between architecture groups is
// unspecified; value order within each group is preserved.
func (a ArchStrings) All() []string {
	var out []string
	for _, values := range a {
		out = appendNonEmpty(out, values...)
	}
	return out
}

type scalarOverrides uint8

const (
	overridePkgDesc scalarOverrides = 1 << iota
	overrideURL
	overrideInstall
	overrideChangelog
)

type SrcinfoPackage struct {
	PkgName    string      `mapstructure:"pkgname" json:"pkgname" yml:"pkgname" toml:"pkgname"`
	PkgDesc    string      `mapstructure:"pkgdesc" json:"pkgdesc" yml:"pkgdesc" toml:"pkgdesc"`
	PkgArch    []string    `mapstructure:"arch" json:"arch" yml:"arch" toml:"arch"`
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

	// overrides distinguishes an absent scalar from an explicitly empty split
	// package override without exposing go-srcinfo's NUL sentinel.
	overrides scalarOverrides
}

// SrcinfoBase represents the pkgbase of a .SRCINFO (build info common to all packages).
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
	CKSums       ArchStrings `mapstructure:"cksums" json:"cksums" yml:"cksums" toml:"cksums"`
	MakeDepends  ArchStrings `mapstructure:"makedepends" json:"makedepends" yml:"makedepends" toml:"makedepends"`
	CheckDepends ArchStrings `mapstructure:"checkdepends" json:"checkdepends" yml:"checkdepends" toml:"checkdepends"`
}

// Version renders the full version "[epoch:]pkgver-pkgrel"; the epoch prefix and
// the pkgrel suffix are dropped when their fields are empty.
func (b *SrcinfoBase) Version() string {
	v := b.PkgVer
	if b.PkgRel != "" {
		v += "-" + b.PkgRel
	}
	if b.PkgEpoch != "" {
		v = b.PkgEpoch + ":" + v
	}
	return v
}

type SRCINFO struct {
	SrcinfoBase    `mapstructure:",squash"`
	SrcinfoPackage `mapstructure:",squash"`
	Packages       []SrcinfoPackage `mapstructure:"packages" json:"packages" yml:"packages" toml:"packages"`
}
