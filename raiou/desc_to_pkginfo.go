package raiou

import (
	"fmt"
	"strings"
)

// ToPKGINFO converts a DESC struct into a PKGINFO struct.
// It maps known fields and also extracts `pkgtype` from XData if available.
func (d *DESC) ToPKGINFO() (*PKGINFO, error) {
	p := &PKGINFO{
		PkgName:   d.Name,
		PkgBase:   d.Base,
		PkgVer:    d.Version,
		PkgDesc:   d.Description,
		URL:       d.URL,
		Arch:      d.Arch,
		Size:      d.Size,
		Packager:  d.Packager,
		License:   append([]string{}, d.License...),
		Replaces:  append([]string{}, d.Replaces...),
		Group:     append([]string{}, d.Groups...),
		Conflict:  append([]string{}, d.Conflicts...),
		Provides:  append([]string{}, d.Provides...),
		Depend:    append([]string{}, d.Depends...),
		OptDepend: append([]string{}, d.OptDepends...),
		XData:     make(map[string]string),
	}

	// Convert BuildDate to Unix timestamp
	p.BuildDate = d.BuildDate.Unix()

	// Extract pkgtype from XData
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
			// Flatten slice into comma-separated string
			p.XData[k] = flattenValues(v)
		}
	}

	return p, nil
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
