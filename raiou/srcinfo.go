package raiou

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/Morganamilo/go-srcinfo"
)

type SrcInfoParser struct {
	parsed  bool
	srcinfo *srcinfo.Srcinfo
}

type Srcinfo = srcinfo.Srcinfo
type Package = srcinfo.Package
type PackageBase = srcinfo.PackageBase
type ArchString = srcinfo.ArchString

// Parse parses a .SRCINFO content and returns the corresponding Srcinfo struct.
func (s *SrcInfoParser) Parse(bs []byte) (*Srcinfo, error) {
	if s.parsed {
		return s.srcinfo, nil
	}

	lines := bytes.Split(bs, []byte{'\n'})
	strLines := make([]string, 0, len(lines))
	for _, line := range lines {
		strLines = append(strLines, string(line))
	}

	kvs, err := parseKeyValues(strLines)
	if err != nil {
		return nil, err
	}

	src := &Srcinfo{}
	var currentPkg *Package // nil means global context (PackageBase/Package)
	seenPkgnames := map[string]struct{}{}
	archDefined := false

	// known fields and their types
	singleFields := map[string]*string{
		"pkgbase":   &src.Pkgbase,
		"pkgver":    &src.Pkgver,
		"pkgrel":    &src.Pkgrel,
		"epoch":     &src.Epoch,
		"url":       &src.URL,
		"install":   &src.Install,
		"changelog": &src.Changelog,
		"pkgdesc":   &src.Pkgdesc,
	}
	stringSliceFields := map[string]*[]string{
		"arch":         &src.Arch,
		"license":      &src.License,
		"groups":       &src.Groups,
		"backup":       &src.Backup,
		"options":      &src.Options,
		"validpgpkeys": &src.ValidPGPKeys,
		"noextract":    &src.NoExtract,
	}
	archStringSliceFields := map[string]*[]srcinfo.ArchString{
		"source":       &src.Source,
		"md5sums":      &src.MD5Sums,
		"sha1sums":     &src.SHA1Sums,
		"sha224sums":   &src.SHA224Sums,
		"sha256sums":   &src.SHA256Sums,
		"sha384sums":   &src.SHA384Sums,
		"sha512sums":   &src.SHA512Sums,
		"b2sums":       &src.B2Sums,
		"depends":      &src.Depends,
		"optdepends":   &src.OptDepends,
		"provides":     &src.Provides,
		"conflicts":    &src.Conflicts,
		"replaces":     &src.Replaces,
		"makedepends":  &src.MakeDepends,
		"checkdepends": &src.CheckDepends,
	}

	for i, kv := range kvs {
		key := kv.Key()
		value := kv.Value()

		if value == "" {
			return nil, fmt.Errorf("empty value for key '%s' on line %d", key, i+1)
		}

		if key == "pkgname" {
			// start new package block
			pkg := Package{Pkgname: value}

			if src.Pkgbase == "" {
				return nil, fmt.Errorf("'pkgname' appears before 'pkgbase' on line %d", i+1)
			}
			if _, ok := seenPkgnames[value]; ok {
				return nil, fmt.Errorf("duplicate 'pkgname' %q on line %d", value, i+1)
			}
			seenPkgnames[value] = struct{}{}

			src.Packages = append(src.Packages, pkg)
			currentPkg = &src.Packages[len(src.Packages)-1]
			continue
		}

		targetSingleFields := singleFields
		targetStringSliceFields := stringSliceFields
		targetArchStringSliceFields := archStringSliceFields

		// if inside package block, override targets
		if currentPkg != nil {
			targetSingleFields = map[string]*string{
				"pkgdesc":   &currentPkg.Pkgdesc,
				"url":       &currentPkg.URL,
				"install":   &currentPkg.Install,
				"changelog": &currentPkg.Changelog,
			}
			targetStringSliceFields = map[string]*[]string{
				"arch":    &currentPkg.Arch,
				"license": &currentPkg.License,
				"groups":  &currentPkg.Groups,
				"backup":  &currentPkg.Backup,
				"options": &currentPkg.Options,
			}
			targetArchStringSliceFields = map[string]*[]ArchString{
				"depends":    &currentPkg.Depends,
				"optdepends": &currentPkg.OptDepends,
				"provides":   &currentPkg.Provides,
				"conflicts":  &currentPkg.Conflicts,
				"replaces":   &currentPkg.Replaces,
			}
		}

		// dispatch
		if ptr, ok := targetSingleFields[key]; ok {
			*ptr = value
		} else if ptr, ok := targetStringSliceFields[key]; ok {
			if key == "arch" && value == "any" {
				return nil, fmt.Errorf("invalid arch 'any' for key '%s' on line %d", key, i+1)
			}
			if key == "arch" {
				archDefined = true
			}
			*ptr = append(*ptr, value)
		} else if ptr, ok := targetArchStringSliceFields[key]; ok {
			as := parseArchString(value)
			if as.Arch != "" && as.Arch != "any" {
				found := false
				for _, a := range src.Arch {
					if a == as.Arch {
						found = true
						break
					}
				}
				if !found {
					return nil, fmt.Errorf("unsupported arch '%s' for key '%s' on line %d", as.Arch, key, i+1)
				}
			} else if as.Arch == "any" {
				return nil, fmt.Errorf("invalid arch 'any' for key '%s' on line %d", key, i+1)
			}
			*ptr = append(*ptr, as)
		} else {
			return nil, fmt.Errorf("unknown key: %s", key)
		}
	}

	if src.Pkgbase == "" {
		return nil, fmt.Errorf("missing required field 'pkgbase'")
	}
	if len(src.Packages) == 0 {
		return nil, fmt.Errorf("missing required field 'pkgname'")
	}
	if src.Pkgver == "" {
		return nil, fmt.Errorf("missing required field 'pkgver'")
	}
	if src.Pkgrel == "" {
		return nil, fmt.Errorf("missing required field 'pkgrel'")
	}
	if !archDefined {
		return nil, fmt.Errorf("missing required field 'arch'")
	}

	s.srcinfo = src
	s.parsed = true
	return src, nil
}

func (s *SrcInfoParser) ParseFile(filename string) (*Srcinfo, error) {
	if s.parsed {
		return s.srcinfo, nil
	}

	bs, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return s.Parse(bs)
}

// parseArchString parses a value that may have optional arch: "arch:value" or "value"
func parseArchString(s string) ArchString {
	if idx := strings.Index(s, ":"); idx != -1 {
		return ArchString{
			Arch:  s[:idx],
			Value: s[idx+1:],
		}
	}
	return ArchString{
		Arch:  "",
		Value: s,
	}
}

func NewSrcInfoParser() *SrcInfoParser {
	return &SrcInfoParser{
		parsed:  false,
		srcinfo: nil,
	}
}
func ParseFile(filename string) (*Srcinfo, error) {
	return NewSrcInfoParser().ParseFile(filename)
}

func Parse(bs []byte) (*Srcinfo, error) {
	return NewSrcInfoParser().Parse(bs)
}
