package repo

import (
	"archive/tar"
	"fmt"
	"io"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/compress"
)

// FilesFromDB parses a repository ".files" archive — a compressed tar of
// "<pkg>-<ver>/desc" and "<pkg>-<ver>/files" members — into a map from package
// name to its file list, mirroring the format the native db writer emits. The
// name comes from the desc %NAME% (not the directory) so it survives a rename.
func FilesFromDB(r io.Reader) (map[string][]string, error) {
	dec, _, err := compress.DetectCompression(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress files db: %w", err)
	}
	defer dec.Close()

	type entry struct {
		name  string
		files []string
	}
	dirs := map[string]*entry{}
	tr := tar.NewReader(dec)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read files db tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		dir, member, ok := splitEntryPath(hdr.Name)
		if !ok {
			if isNewDBFormat(hdr, tr) {
				return nil, fmt.Errorf("%w: %q is a SQLite pacman.db", ErrUnsupportedDBFormat, hdr.Name)
			}
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", hdr.Name, err)
		}
		e := dirs[dir]
		if e == nil {
			e = &entry{}
			dirs[dir] = e
		}
		switch member {
		case "desc":
			e.name = descName(data)
		case "files":
			e.files = parseFilesMember(data)
		}
	}
	out := make(map[string][]string, len(dirs))
	for _, e := range dirs {
		if e.name != "" {
			out[e.name] = e.files
		}
	}
	return out, nil
}

// parseFilesMember extracts the path list from a "%FILES%" member, dropping the
// header line and any blank lines.
func parseFilesMember(data []byte) []string {
	lines := strings.Split(string(data), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" || line == "%FILES%" {
			continue
		}
		files = append(files, line)
	}
	return files
}
