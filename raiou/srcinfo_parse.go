package raiou

import (
	"io"

	"github.com/Morganamilo/go-srcinfo"
)

type GoSRCINFO srcinfo.Srcinfo

func ParseSrcinfo(r io.Reader) (*GoSRCINFO, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	srcinfo, err := srcinfo.Parse(string(b))
	if err != nil {
		return nil, err
	}
	return (*GoSRCINFO)(srcinfo), nil
}

func ParseSrcinfoFile(path string) (*GoSRCINFO, error) {
	srcinfo, err := srcinfo.ParseFile(path)
	if err != nil {
		return nil, err
	}
	return (*GoSRCINFO)(srcinfo), nil
}

func ParseSrcinfoString(data string) (*GoSRCINFO, error) {
	srcinfo, err := srcinfo.Parse(data)
	if err != nil {
		return nil, err
	}
	return (*GoSRCINFO)(srcinfo), nil
}
