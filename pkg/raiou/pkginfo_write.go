package raiou

import (
	"sort"
	"strconv"
	"strings"
)

// Bytes serializes the PKGINFO to the `.PKGINFO` text format makepkg emits and
// ParsePkginfo reads back. pkgname/pkgver/arch/builddate/size are always written
// (pacman requires them); other scalars are omitted when empty, matching how the
// parser treats an empty value as absent. Repeated and xdata fields keep a stable
// order so the output is reproducible.
func (p *PKGINFO) Bytes() []byte {
	var b strings.Builder
	write := func(key, value string) {
		if value == "" {
			return
		}
		b.WriteString(key)
		b.WriteString(" = ")
		b.WriteString(value)
		b.WriteByte('\n')
	}
	writeAll := func(key string, values []string) {
		for _, v := range values {
			write(key, v)
		}
	}

	write("pkgname", p.PkgName)
	write("pkgbase", p.PkgBase)
	write("pkgver", p.PkgVer)
	write("pkgdesc", p.PkgDesc)
	write("url", p.URL)
	// builddate/size are numeric and mandatory, so they are written unconditionally.
	write("builddate", strconv.FormatInt(p.BuildDate, 10))
	write("packager", p.Packager)
	write("size", strconv.FormatInt(p.Size, 10))
	write("arch", p.Arch)
	writeAll("license", p.License)
	writeAll("replaces", p.Replaces)
	writeAll("group", p.Group)
	writeAll("conflict", p.Conflict)
	writeAll("provides", p.Provides)
	writeAll("backup", p.Backup)
	writeAll("depend", p.Depend)
	writeAll("optdepend", p.OptDepend)
	writeAll("makedepend", p.MakeDepend)
	writeAll("checkdepend", p.CheckDepend)

	// pkgtype leads the xdata block (as makepkg writes it); remaining xdata keys
	// follow in sorted order for a deterministic result.
	if p.PkgType != "" {
		write("xdata", "pkgtype="+p.PkgType)
	}
	keys := make([]string, 0, len(p.XData))
	for k := range p.XData {
		if k == "pkgtype" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		write("xdata", k+"="+p.XData[k])
	}

	return []byte(b.String())
}
