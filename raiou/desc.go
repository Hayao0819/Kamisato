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
	Name        string
	Version     string
	Base        string
	Description string
	URL         string
	Arch        string
	BuildDate   time.Time
	InstallDate time.Time
	Packager    string
	Size        int64
	Reason      int64
	Groups      []string
	License     []string
	Validation  string
	Replaces    []string
	Depends     []string
	OptDepends  []string
	Conflicts   []string
	Provides    []string
	XData       []keyValue
	ExtraFields map[string][]string
}

func NewDESC() *DESC {
	return &DESC{
		License:     []string{},
		Depends:     []string{},
		Provides:    []string{},
		XData:       []keyValue{},
		ExtraFields: make(map[string][]string),
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
			desc.Name = strings.Join(buffer, " ")
		case "VERSION":
			desc.Version = strings.Join(buffer, " ")
		case "BASE":
			desc.Base = strings.Join(buffer, " ")
		case "DESC":
			desc.Description = strings.Join(buffer, "\n")
		case "URL":
			desc.URL = strings.Join(buffer, " ")
		case "ARCH":
			desc.Arch = strings.Join(buffer, " ")
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
			desc.Packager = strings.Join(buffer, " ")
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
			desc.Validation = strings.Join(buffer, " ")
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
			buffer = append(buffer, line)
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
