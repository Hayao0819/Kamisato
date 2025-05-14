package raiou

import (
	"io"

	"github.com/Morganamilo/go-srcinfo"
)

type Srcinfo srcinfo.Srcinfo

func ParseSrcinfo(r io.Reader) (*Srcinfo, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	srcinfo, err := srcinfo.Parse(string(b))
	if err != nil {
		return nil, err
	}
	return (*Srcinfo)(srcinfo), nil
}

func ParseSrcinfoFile(path string) (*Srcinfo, error) {
	srcinfo, err := srcinfo.ParseFile(path)
	if err != nil {
		return nil, err
	}
	return (*Srcinfo)(srcinfo), nil
}

func ParseSrcinfoString(data string) (*Srcinfo, error) {
	srcinfo, err := srcinfo.Parse(data)
	if err != nil {
		return nil, err
	}
	return (*Srcinfo)(srcinfo), nil
}
