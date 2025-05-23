package raiou

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type DESC struct {
	Name        string              `json:"name" yml:"name" toml:"name"`
	Version     string              `json:"version" yml:"version" toml:"version"`
	Base        string              `json:"base" yml:"base" toml:"base"`
	Description string              `json:"description" yml:"description" toml:"description"`
	URL         string              `json:"url" yml:"url" toml:"url"`
	Arch        string              `json:"arch" yml:"arch" toml:"arch"`
	BuildDate   time.Time           `json:"builddate" yml:"builddate" toml:"builddate"`
	InstallDate time.Time           `json:"installdate" yml:"installdate" toml:"installdate"`
	Packager    string              `json:"packager" yml:"packager" toml:"packager"`
	Size        int64               `json:"size" yml:"size" toml:"size"`
	Reason      int64               `json:"reason" yml:"reason" toml:"reason"`
	Groups      []string            `json:"groups" yml:"groups" toml:"groups"`
	License     []string            `json:"license" yml:"license" toml:"license"`
	Validation  string              `json:"validation" yml:"validation" toml:"validation"`
	Replaces    []string            `json:"replaces" yml:"replaces" toml:"replaces"`
	Depends     []string            `json:"depends" yml:"depends" toml:"depends"`
	OptDepends  []string            `json:"optdepends" yml:"optdepends" toml:"optdepends"`
	Conflicts   []string            `json:"conflicts" yml:"conflicts" toml:"conflicts"`
	Provides    []string            `json:"provides" yml:"provides" toml:"provides"`
	XData       []keyValue          `json:"xdata" yml:"xdata" toml:"xdata"`
	ExtraFields map[string][]string `json:"extrafields" yml:"extrafields" toml:"extrafields"`
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

func ParseDescFile(path string) (*DESC, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening desc file: %w", err)
	}
	defer f.Close()
	return ParseDesc(f)
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
		case "NAME":
			if len(buffer) > 0 {
				desc.Name = buffer[0]
			}
		case "VERSION":
			if len(buffer) > 0 {
				desc.Version = buffer[0]
			}
		case "BASE":
			if len(buffer) > 0 {
				desc.Base = buffer[0]
			}
		case "DESC":
			desc.Description = strings.Join(buffer, "\n")
		case "URL":
			if len(buffer) > 0 {
				desc.URL = buffer[0]
			}
		case "ARCH":
			if len(buffer) > 0 {
				desc.Arch = buffer[0]
			}
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
			if len(buffer) > 0 {
				desc.Packager = buffer[0]
			}
		case "SIZE":
			if s, err := parseInt(buffer); err != nil {
				return fmt.Errorf("invalid SIZE: %w", err)
			} else {
				desc.Size = s
			}
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
			if len(buffer) > 0 {
				desc.Validation = buffer[0]
			}
		case "REPLACES":
			desc.Replaces = append(desc.Replaces, buffer...)
		case "DEPENDS":
			desc.Depends = append(desc.Depends, buffer...)
		case "OPTDEPENDS":
			desc.OptDepends = append(desc.OptDepends, buffer...)
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
			buffer = append(buffer, strings.TrimSpace(line))
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
