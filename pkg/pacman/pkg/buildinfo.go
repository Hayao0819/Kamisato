package pkg

import (
	"archive/tar"
	"fmt"
	"io"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// ErrBuildInfoNotFound signals a package archive with no .BUILDINFO member. A
// provenance gate treats it as a hard failure: a package built outside the
// expected sandbox, or a hand-crafted archive.
var ErrBuildInfoNotFound = fmt.Errorf(".BUILDINFO not found")

// ReadBuildInfo extracts and parses the .BUILDINFO member from a package archive,
// returning ErrBuildInfoNotFound when the archive has none.
func ReadBuildInfo(r io.Reader) (*raiou.BUILDINFO, error) {
	var data string
	found := false
	err := walkPackageTar(r, func(hdr *tar.Header, content io.Reader) (bool, error) {
		if hdr.Name != ".BUILDINFO" {
			return false, nil
		}
		buf := new(strings.Builder)
		if _, err := io.Copy(buf, content); err != nil {
			return false, fmt.Errorf("failed to read .BUILDINFO: %w", err)
		}
		data = buf.String()
		found = true
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrBuildInfoNotFound
	}
	return raiou.ParseBuildinfo(strings.NewReader(data))
}
