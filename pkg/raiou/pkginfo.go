package raiou

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// PKGINFO represents the parsed PKGINFO file.
type PKGINFO struct {
	PkgName     string            `json:"pkgname" yml:"pkgname" toml:"pkgname"`
	PkgBase     string            `json:"pkgbase" yml:"pkgbase" toml:"pkgbase"`
	PkgVer      string            `json:"pkgver" yml:"pkgver" toml:"pkgver"`
	PkgDesc     string            `json:"pkgdesc" yml:"pkgdesc" toml:"pkgdesc"`
	URL         string            `json:"url" yml:"url" toml:"url"`
	BuildDate   int64             `json:"builddate" yml:"builddate" toml:"builddate"`
	Packager    string            `json:"packager" yml:"packager" toml:"packager"`
	Size        int64             `json:"size" yml:"size" toml:"size"`
	Arch        string            `json:"arch" yml:"arch" toml:"arch"`
	License     []string          `json:"license" yml:"license" toml:"license"`
	Replaces    []string          `json:"replaces" yml:"replaces" toml:"replaces"`
	Group       []string          `json:"group" yml:"group" toml:"group"`
	Conflict    []string          `json:"conflict" yml:"conflict" toml:"conflict"`
	Provides    []string          `json:"provides" yml:"provides" toml:"provides"`
	Backup      []string          `json:"backup" yml:"backup" toml:"backup"`
	Depend      []string          `json:"depend" yml:"depend" toml:"depend"`
	OptDepend   []string          `json:"optdepend" yml:"optdepend" toml:"optdepend"`
	MakeDepend  []string          `json:"makedepend" yml:"makedepend" toml:"makedepend"`
	CheckDepend []string          `json:"checkdepend" yml:"checkdepend" toml:"checkdepend"`
	XData       map[string]string `json:"xdata" yml:"xdata" toml:"xdata"`
	PkgType     string            `json:"pkgtype" yml:"pkgtype" toml:"pkgtype"`
}

// NewPKGINFO creates a new PKGINFO struct.
func NewPKGINFO() *PKGINFO {
	return &PKGINFO{
		License:     make([]string, 0),
		Replaces:    make([]string, 0),
		Group:       make([]string, 0),
		Conflict:    make([]string, 0),
		Provides:    make([]string, 0),
		Backup:      make([]string, 0),
		Depend:      make([]string, 0),
		OptDepend:   make([]string, 0),
		MakeDepend:  make([]string, 0),
		CheckDepend: make([]string, 0),
		XData:       make(map[string]string),
	}
}

func ParsePkginfoFile(path string) (*PKGINFO, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer r.Close()
	return ParsePkginfo(r)
}
func ParsePkginfoString(data string) (*PKGINFO, error) {
	r := strings.NewReader(data)
	return ParsePkginfo(r)
}

// Parse reads a PKGINFO file from the given io.Reader and returns a PKGINFO struct.
func ParsePkginfo(r io.Reader) (*PKGINFO, error) {
	p := NewPKGINFO()
	lines, err := readLines(r)
	if err != nil {
		return nil, err
	}

	keyValues, err := parseKeyValues(lines)
	if err != nil {
		return nil, fmt.Errorf("error parsing key-value pairs: %w", err)
	}

	if err := p.parseKeyValues(keyValues); err != nil {
		return nil, err
	}

	if p.PkgType == "" {
		return nil, fmt.Errorf("missing pkgtype in xdata")
	}

	return p, nil
}

func (p *PKGINFO) parseKeyValues(kvs []keyValue) error {
	for _, kv := range kvs {
		key := kv.Key()
		value := kv.Value()

		switch key {
		case "pkgname":
			p.PkgName = value
		case "pkgbase":
			p.PkgBase = value
		case "pkgver":
			p.PkgVer = value
		case "pkgdesc":
			p.PkgDesc = value
		case "url":
			p.URL = value
		case "builddate":
			buildDate, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid builddate: %s", value)
			}
			p.BuildDate = buildDate
		case "packager":
			p.Packager = value
		case "size":
			size, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid size: %s", value)
			}
			p.Size = size
		case "arch":
			p.Arch = value
		case "license":
			p.License = append(p.License, value)
		case "replaces":
			p.Replaces = append(p.Replaces, value)
		case "group":
			p.Group = append(p.Group, value)
		case "conflict":
			p.Conflict = append(p.Conflict, value)
		case "provides":
			p.Provides = append(p.Provides, value)
		case "backup":
			p.Backup = append(p.Backup, value)
		case "depend":
			p.Depend = append(p.Depend, value)
		case "optdepend":
			p.OptDepend = append(p.OptDepend, value)
		case "makedepend":
			p.MakeDepend = append(p.MakeDepend, value)
		case "checkdepend":
			p.CheckDepend = append(p.CheckDepend, value)
		case "xdata":
			if err := p.parseXData(value); err != nil {
				return err
			}
		default:
			slog.Warn("unknown key in PKGINFO", "key", key, "value", value)
		}
	}
	return nil
}

func (p *PKGINFO) parseXData(data string) error {
	parts := strings.SplitN(data, "=", 2)
	if len(parts) == 2 {
		xKey := strings.TrimSpace(parts[0])
		xValue := strings.TrimSpace(parts[1])
		if xKey == "pkgtype" {
			p.PkgType = xValue
		}
		p.XData[xKey] = xValue
		return nil
	}
	return fmt.Errorf("invalid xdata format: %s", data)
}
