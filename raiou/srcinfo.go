package raiou

type ArchString struct {
	Arch  string
	Value string
}

type Package struct {
	PkgName    string
	PkgDesc    string
	PkgArch    []string
	URL        string
	License    []string
	Groups     []string
	Depends    []ArchString
	OptDepends []ArchString
	Provides   []ArchString
	Conflicts  []ArchString
	Replaces   []ArchString
	Backup     []string
	Options    []string
	Install    string
	Changelog  string
}

type PackageBase struct {
	PkgBase      string
	PkgVer       string
	PkgRel       string
	PkgEpoch     string // Renamed Epoch
	Source       []ArchString
	ValidPGPKeys []string
	NoExtract    []string
	MD5Sums      []ArchString
	SHA1Sums     []ArchString
	SHA224Sums   []ArchString
	SHA256Sums   []ArchString
	SHA384Sums   []ArchString
	SHA512Sums   []ArchString
	B2Sums       []ArchString
	MakeDepends  []ArchString
	CheckDepends []ArchString
}

type SRCINFO struct {
	PackageBase
	Package
	Packages []Package
}
