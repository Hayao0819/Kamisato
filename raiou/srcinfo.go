package raiou

import (
	"bytes"
	"fmt"
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

	// known fields and their types
	singleFields := map[string]*string{
		"pkgbase":   &src.Pkgbase,
		"pkgver":    &src.Pkgver,
		"pkgrel":    &src.Pkgrel,
		"epoch":     &src.Epoch,
		"url":       &src.URL,
		"install":   &src.Install,
		"changelog": &src.Changelog,
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
	archStringSliceFields := map[string]*[]ArchString{
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

	for _, kv := range kvs {
		key := kv.Key()
		value := kv.Value()

		if key == "pkgname" {
			// start new package block
			pkg := Package{Pkgname: value}
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
			*ptr = append(*ptr, value)
		} else if ptr, ok := targetArchStringSliceFields[key]; ok {
			as := parseArchString(value)
			*ptr = append(*ptr, as)
		} else {
			return nil, fmt.Errorf("unknown key: %s", key)
		}
	}

	s.srcinfo = src
	s.parsed = true
	return src, nil
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
