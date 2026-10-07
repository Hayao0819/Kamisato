package pkg

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// ErrBuildInfoNotFound signals a package archive with no .BUILDINFO member. A
// provenance gate treats it as a hard failure: a package built outside the
// expected sandbox, or a hand-crafted archive.
var ErrBuildInfoNotFound = fmt.Errorf(".BUILDINFO not found")

// ReadBuildInfo extracts and parses the .BUILDINFO member from a package archive,
// returning ErrBuildInfoNotFound when the archive has none.
func ReadBuildInfo(r io.Reader) (*raiou.BUILDINFO, error) {
	data, err := ReadBuildInfoData(r)
	if err != nil {
		return nil, err
	}
	return raiou.ParseBuildinfo(bytes.NewReader(data))
}

func ReadBuildInfoData(r io.Reader) ([]byte, error) {
	var data []byte
	found := false
	err := walkPackageTar(r, func(hdr *tar.Header, content io.Reader) (bool, error) {
		if hdr.Name != ".BUILDINFO" {
			return false, nil
		}
		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, content); err != nil {
			return false, fmt.Errorf("failed to read .BUILDINFO: %w", err)
		}
		data = buf.Bytes()
		found = true
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrBuildInfoNotFound
	}
	return data, nil
}
