package raiou

import (
	"io"

	"github.com/Morganamilo/go-srcinfo"
)

type SRCINFO srcinfo.Srcinfo

func ParseSrcinfo(r io.Reader) (*SRCINFO, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	srcinfo, err := srcinfo.Parse(string(b))
	if err != nil {
		return nil, err
	}
	return (*SRCINFO)(srcinfo), nil
}

func ParseSrcinfoFile(path string) (*SRCINFO, error) {
	srcinfo, err := srcinfo.ParseFile(path)
	if err != nil {
		return nil, err
	}
	return (*SRCINFO)(srcinfo), nil
}

func ParseSrcinfoString(data string) (*SRCINFO, error) {
	srcinfo, err := srcinfo.Parse(data)
	if err != nil {
		return nil, err
	}
	return (*SRCINFO)(srcinfo), nil
}
