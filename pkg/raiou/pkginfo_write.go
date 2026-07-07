package raiou

import (
	"sort"
	"strconv"
	"strings"
)

// Bytes serializes PKGINFO to the .PKGINFO text format. Required fields (pkgname/pkgver/arch/builddate/size)
// are always written; empty scalars are omitted; repeated/xdata fields keep a stable order.
func (p *PKGINFO) Bytes() []byte {
	var b strings.Builder
	// Newlines in a value would inject spurious key lines; collapse to spaces.
	sanitize := strings.NewReplacer("\r", " ", "\n", " ")
	write := func(key, value string) {
		if value == "" {
			return
		}
		b.WriteString(key)
		b.WriteString(" = ")
		b.WriteString(sanitize.Replace(value))
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
	// builddate/size are mandatory numeric fields; write unconditionally.
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
