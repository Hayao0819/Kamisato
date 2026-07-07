package raiou

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DESC struct {
	FileName     string              `json:"filename" yml:"filename" toml:"filename"`
	Name         string              `json:"name" yml:"name" toml:"name"`
	Version      string              `json:"version" yml:"version" toml:"version"`
	Base         string              `json:"base" yml:"base" toml:"base"`
	Description  string              `json:"description" yml:"description" toml:"description"`
	URL          string              `json:"url" yml:"url" toml:"url"`
	Arch         string              `json:"arch" yml:"arch" toml:"arch"`
	BuildDate    time.Time           `json:"builddate" yml:"builddate" toml:"builddate"`
	InstallDate  time.Time           `json:"installdate" yml:"installdate" toml:"installdate"`
	Packager     string              `json:"packager" yml:"packager" toml:"packager"`
	Size         int64               `json:"size" yml:"size" toml:"size"`
	ISize        int64               `json:"isize" yml:"isize" toml:"isize"`
	CSize        int64               `json:"csize" yml:"csize" toml:"csize"`
	Reason       int64               `json:"reason" yml:"reason" toml:"reason"`
	Groups       []string            `json:"groups" yml:"groups" toml:"groups"`
	License      []string            `json:"license" yml:"license" toml:"license"`
	Validation   string              `json:"validation" yml:"validation" toml:"validation"`
	InstalledDB  string              `json:"installeddb" yml:"installeddb" toml:"installeddb"`
	Replaces     []string            `json:"replaces" yml:"replaces" toml:"replaces"`
	Depends      []string            `json:"depends" yml:"depends" toml:"depends"`
	OptDepends   []string            `json:"optdepends" yml:"optdepends" toml:"optdepends"`
	MakeDepends  []string            `json:"makedepends" yml:"makedepends" toml:"makedepends"`
	CheckDepends []string            `json:"checkdepends" yml:"checkdepends" toml:"checkdepends"`
	SHA256SUM    string              `json:"sha256sum" yml:"sha256sum" toml:"sha256sum"`
	MD5SUM       string              `json:"md5sum" yml:"md5sum" toml:"md5sum"`
	PGPSIG       string              `json:"pgpsig" yml:"pgpsig" toml:"pgpsig"`
	Conflicts    []string            `json:"conflicts" yml:"conflicts" toml:"conflicts"`
	Provides     []string            `json:"provides" yml:"provides" toml:"provides"`
	XData        []keyValue          `json:"xdata" yml:"xdata" toml:"xdata"`
	ExtraFields  map[string][]string `json:"extrafields" yml:"extrafields" toml:"extrafields"`
}

func NewDESC() *DESC {
	return &DESC{
		Groups:      []string{},
		License:     []string{},
		Replaces:    []string{},
		Depends:     []string{},
		OptDepends:  []string{},
		Conflicts:   []string{},
		Provides:    []string{},
		XData:       []keyValue{},
		ExtraFields: map[string][]string{},
	}
}

func ParseDescString(data string) (*DESC, error) {
	return ParseDesc(strings.NewReader(data))
}

func ParseDesc(r io.Reader) (*DESC, error) {
	desc := NewDESC()
	scanner := bufio.NewScanner(r)

	var currentField string
	var buffer []string

	flush := func() error {
		if currentField == "" {
			return nil
		}

		switch currentField {
		case "FILENAME":
			desc.FileName = first(buffer)
		case "NAME":
			desc.Name = first(buffer)
		case "VERSION":
			desc.Version = first(buffer)
		case "BASE":
			desc.Base = first(buffer)
		case "DESC":
			desc.Description = strings.Join(buffer, "\n")
		case "URL":
			desc.URL = first(buffer)
		case "ARCH":
			desc.Arch = first(buffer)
		case "BUILDDATE":
			if t, err := parseUnixTimestamp(buffer); err != nil {
				return fmt.Errorf("invalid BUILDDATE: %w", err)
			} else {
				desc.BuildDate = t
			}
		case "INSTALLDATE":
			if t, err := parseUnixTimestamp(buffer); err != nil {
				return fmt.Errorf("invalid INSTALLDATE: %w", err)
			} else {
				desc.InstallDate = t
			}
		case "PACKAGER":
			desc.Packager = first(buffer)
		case "SIZE":
			if s, err := parseInt(buffer); err != nil {
				return fmt.Errorf("invalid SIZE: %w", err)
			} else {
				desc.Size = s
			}
		case "ISIZE":
			if s, err := parseInt(buffer); err != nil {
				return fmt.Errorf("invalid ISIZE: %w", err)
			} else {
				desc.ISize = s
			}
		case "CSIZE":
			if s, err := parseInt(buffer); err != nil {
				return fmt.Errorf("invalid CSIZE: %w", err)
			} else {
				desc.CSize = s
			}
		case "SHA256SUM":
			desc.SHA256SUM = first(buffer)
		case "MD5SUM":
			desc.MD5SUM = first(buffer)
		case "PGPSIG":
			desc.PGPSIG = first(buffer)
		case "REASON":
			if r, err := parseInt(buffer); err != nil {
				return fmt.Errorf("invalid REASON: %w", err)
			} else {
				desc.Reason = r
			}
		case "GROUPS":
			desc.Groups = append(desc.Groups, buffer...)
		case "LICENSE":
			desc.License = append(desc.License, buffer...)
		case "VALIDATION":
			desc.Validation = first(buffer)
		case "INSTALLED_DB":
			// CachyOS's libalpm records the source repo in each local db entry
			// (be_local.c). It never appears in a sync/repo db, so it is read
			// here but not emitted by the repo-add writer.
			desc.InstalledDB = first(buffer)
		case "REPLACES":
			desc.Replaces = append(desc.Replaces, buffer...)
		case "DEPENDS":
			desc.Depends = append(desc.Depends, buffer...)
		case "OPTDEPENDS":
			desc.OptDepends = append(desc.OptDepends, buffer...)
		case "MAKEDEPENDS":
			desc.MakeDepends = append(desc.MakeDepends, buffer...)
		case "CHECKDEPENDS":
			desc.CheckDepends = append(desc.CheckDepends, buffer...)
		case "CONFLICTS":
			desc.Conflicts = append(desc.Conflicts, buffer...)
		case "PROVIDES":
			desc.Provides = append(desc.Provides, buffer...)
		case "XDATA":
			kvPairs, err := parseKeyValues(buffer)
			if err != nil {
				return fmt.Errorf("failed to parse XDATA: %w", err)
			}
			desc.XData = kvPairs
		default:
			slog.Warn("unknown field in desc file", "field", currentField, "values", buffer)
			desc.ExtraFields[currentField] = append(desc.ExtraFields[currentField], buffer...)
		}
		buffer = nil
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "%") && strings.HasSuffix(line, "%") {
			if err := flush(); err != nil {
				return nil, err
			}
			currentField = strings.Trim(line, "%")
		} else if currentField != "" {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				buffer = append(buffer, trimmed)
			}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed reading desc file: %w", err)
	}
	return desc, nil
}

func (d *DESC) ToPKGINFO() (*PKGINFO, error) {
	p := &PKGINFO{
		PkgName:     d.Name,
		PkgBase:     d.Base,
		PkgVer:      d.Version,
		PkgDesc:     d.Description,
		URL:         d.URL,
		Arch:        d.Arch,
		Size:        d.Size,
		Packager:    d.Packager,
		License:     append([]string{}, d.License...),
		Replaces:    append([]string{}, d.Replaces...),
		Group:       append([]string{}, d.Groups...),
		Conflict:    append([]string{}, d.Conflicts...),
		Provides:    append([]string{}, d.Provides...),
		Backup:      []string{},
		Depend:      append([]string{}, d.Depends...),
		OptDepend:   append([]string{}, d.OptDepends...),
		MakeDepend:  append([]string{}, d.MakeDepends...),
		CheckDepend: append([]string{}, d.CheckDepends...),
		XData:       make(map[string]string),
	}

	p.BuildDate = d.BuildDate.Unix()

	for _, kv := range d.XData {
		if kv.Key() == "pkgtype" {
			p.PkgType = kv.Value()
		} else {
			p.XData[kv.Key()] = kv.Value()
		}
	}

	// ExtraFields may contain additional values we can't map — store as XData
	for k, v := range d.ExtraFields {
		if _, exists := p.XData[k]; !exists {
			p.XData[k] = flattenValues(v)
		}
	}

	return p, nil
}

func first(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func flattenValues(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	default:
		return fmt.Sprintf("[%s]", joinQuoted(values, ", "))
	}
}

func joinQuoted(values []string, sep string) string {
	q := make([]string, len(values))
	for i, v := range values {
		q[i] = fmt.Sprintf("%q", v)
	}
	return fmt.Sprint(strings.Join(q, sep))
}

// DescFromPkginfo builds a repository desc entry from a package's .PKGINFO plus
// the three values repo-add computes outside it: the stored filename, the
// package file's own (compressed) size, and its sha256. PGPSIG is left empty;
// the caller sets it from the package's detached signature when one exists.
func DescFromPkginfo(info *PKGINFO, filename string, csize int64, sha256sum string) *DESC {
	d := NewDESC()
	d.FileName = filename
	d.Name = info.PkgName
	d.Base = info.PkgBase
	d.Version = info.PkgVer
	d.Description = info.PkgDesc
	d.Groups = append([]string{}, info.Group...)
	d.CSize = csize
	d.ISize = info.Size
	d.SHA256SUM = sha256sum
	d.URL = info.URL
	d.License = append([]string{}, info.License...)
	d.Arch = info.Arch
	d.BuildDate = time.Unix(info.BuildDate, 0)
	d.Packager = info.Packager
	d.Replaces = append([]string{}, info.Replaces...)
	d.Conflicts = append([]string{}, info.Conflict...)
	d.Provides = append([]string{}, info.Provides...)
	d.Depends = append([]string{}, info.Depend...)
	d.OptDepends = append([]string{}, info.OptDepend...)
	d.MakeDepends = append([]string{}, info.MakeDepend...)
	d.CheckDepends = append([]string{}, info.CheckDepend...)
	return d
}

// Bytes serializes the desc in repo-add's exact field order and format.
func (d *DESC) Bytes() []byte {
	var b bytes.Buffer
	d.appendTo(&b)
	return b.Bytes()
}

// appendTo writes each %FIELD% block in the order repo-add's db_write_entry
// uses. Empty fields are omitted (format_entry skips a field whose first value
// is empty), and CSIZE/ISIZE/BUILDDATE are written only when set.
func (d *DESC) appendTo(b *bytes.Buffer) {
	writeDescField(b, "FILENAME", d.FileName)
	writeDescField(b, "NAME", d.Name)
	writeDescField(b, "BASE", d.Base)
	writeDescField(b, "VERSION", d.Version)
	writeDescField(b, "DESC", d.Description)
	writeDescField(b, "GROUPS", d.Groups...)
	// CSIZE and ISIZE are always emitted, even when 0: repo-add's format_entry
	// gates on a non-empty string, and a payload-less metapackage legitimately
	// has size=0 (so %ISIZE%\n0). Only an absent field is omitted.
	writeDescField(b, "CSIZE", strconv.FormatInt(d.CSize, 10))
	writeDescField(b, "ISIZE", strconv.FormatInt(d.ISize, 10))
	writeDescField(b, "SHA256SUM", d.SHA256SUM)
	writeDescField(b, "PGPSIG", d.PGPSIG)
	writeDescField(b, "URL", d.URL)
	writeDescField(b, "LICENSE", d.License...)
	writeDescField(b, "ARCH", d.Arch)
	writeDescField(b, "BUILDDATE", buildDateValue(d.BuildDate))
	writeDescField(b, "PACKAGER", d.Packager)
	writeDescField(b, "REPLACES", d.Replaces...)
	writeDescField(b, "CONFLICTS", d.Conflicts...)
	writeDescField(b, "PROVIDES", d.Provides...)
	writeDescField(b, "DEPENDS", d.Depends...)
	writeDescField(b, "OPTDEPENDS", d.OptDepends...)
	writeDescField(b, "MAKEDEPENDS", d.MakeDepends...)
	writeDescField(b, "CHECKDEPENDS", d.CheckDepends...)
}

// descWhitespace collapses runs of whitespace to a single space, matching
// repo-add's `${val//+([[:space:]])/ }` normalization at read time.
var descWhitespace = regexp.MustCompile(`[[:space:]]+`)

func writeDescField(b *bytes.Buffer, field string, values ...string) {
	if len(values) == 0 || values[0] == "" {
		return
	}
	b.WriteByte('%')
	b.WriteString(field)
	b.WriteString("%\n")
	for _, v := range values {
		b.WriteString(descWhitespace.ReplaceAllString(v, " "))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
}

func buildDateValue(t time.Time) string {
	if u := t.Unix(); u > 0 {
		return strconv.FormatInt(u, 10)
	}
	return ""
}
