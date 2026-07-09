package repo

import (
	"archive/tar"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// dbEntry is one package's stored database entry. desc and files hold the raw
// bytes of the "<pkg>-<ver>/desc" and "<pkg>-<ver>/files" members. For packages
// the builder is not modifying these are preserved verbatim from the loaded
// archives, so untouched entries — and in particular their files lists, which
// the desc parser would otherwise discard — survive a read-modify-write intact.
type dbEntry struct {
	dir   string
	name  string
	desc  []byte
	files []byte
}

// dbBuilder assembles the pacman ".db" (desc only) and ".files" (desc + files)
// archives natively, replacing the repo-add CLI. The usual cycle is: LoadDB +
// LoadFiles to seed from the existing archives, Upsert or Remove to mutate, then
// WriteDB + WriteFiles to emit the new archives.
type dbBuilder struct {
	entries map[string]*dbEntry // keyed by dir "<pkg>-<ver>"
}

func newDBBuilder() *dbBuilder {
	return &dbBuilder{entries: map[string]*dbEntry{}}
}

// LoadDB reads a ".db" archive (desc-only) into the builder. A nil reader is a
// no-op, so a fresh repository can be built from nothing.
func (b *dbBuilder) LoadDB(r io.Reader) error {
	return b.load(r, false)
}

// LoadFiles reads a ".files" archive (desc + files) into the builder, capturing
// each package's files member so a rewrite preserves it.
func (b *dbBuilder) LoadFiles(r io.Reader) error {
	return b.load(r, true)
}

func (b *dbBuilder) load(r io.Reader, withFiles bool) error {
	if r == nil {
		return nil
	}
	dec, _, err := pkg.DetectCompression(r)
	if err != nil {
		return fmt.Errorf("failed to decompress db: %w", err)
	}
	defer dec.Close()

	tr := tar.NewReader(dec)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read db tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		dir, member, ok := splitEntryPath(hdr.Name)
		if !ok {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", hdr.Name, err)
		}
		e := b.entries[dir]
		if e == nil {
			e = &dbEntry{dir: dir}
			b.entries[dir] = e
		}
		switch member {
		case "desc":
			e.desc = data
			e.name = descName(data)
		case "files":
			if withFiles {
				e.files = data
			}
		}
	}
	return nil
}

// Upsert adds (or replaces) a package's entry. Any existing entry with the same
// package name is removed first, matching repo-add, which drops all prior
// entries for a name before writing the new one. A non-empty sig is the package's
// detached (binary) signature, embedded as the desc %PGPSIG% like repo-add's
// --include-sigs.
func (b *dbBuilder) Upsert(meta *pkg.BinaryPackageMeta, sig []byte) error {
	if meta == nil || meta.Info == nil {
		return fmt.Errorf("nil package metadata")
	}
	name := meta.Info.PkgName
	ver := meta.Info.PkgVer
	if name == "" || ver == "" {
		return fmt.Errorf("package is missing pkgname or pkgver")
	}
	b.Remove(name)

	desc := raiou.DescFromPkginfo(meta.Info, meta.Filename, meta.CSize, meta.SHA256)
	if len(sig) > 0 {
		desc.PGPSIG = base64.StdEncoding.EncodeToString(sig)
	}

	dir := name + "-" + ver
	b.entries[dir] = &dbEntry{
		dir:   dir,
		name:  name,
		desc:  desc.Bytes(),
		files: filesEntry(meta.Files),
	}
	return nil
}

// Remove drops every entry whose package name matches, reporting whether any
// were removed.
func (b *dbBuilder) Remove(name string) bool {
	removed := false
	for dir, e := range b.entries {
		if e.name == name {
			delete(b.entries, dir)
			removed = true
		}
	}
	return removed
}

// WriteDB writes the gzipped ".db" archive (desc members only).
func (b *dbBuilder) WriteDB(w io.Writer) error {
	return b.write(w, false)
}

// WriteFiles writes the gzipped ".files" archive (desc + files members).
func (b *dbBuilder) WriteFiles(w io.Writer) error {
	return b.write(w, true)
}

func (b *dbBuilder) write(w io.Writer, withFiles bool) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	for _, dir := range b.sortedDirs() {
		e := b.entries[dir]
		if err := writeTarDir(tw, e.dir+"/"); err != nil {
			return err
		}
		if err := writeTarFile(tw, e.dir+"/desc", e.desc); err != nil {
			return err
		}
		if withFiles && e.files != nil {
			if err := writeTarFile(tw, e.dir+"/files", e.files); err != nil {
				return err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to close gzip: %w", err)
	}
	return nil
}

// names returns the package names currently held (the %NAME% of each entry),
// used by Merge to decide which upstream entries a local overlay shadows.
func (b *dbBuilder) names() []string {
	out := make([]string, 0, len(b.entries))
	for _, e := range b.entries {
		if e.name != "" {
			out = append(out, e.name)
		}
	}
	return out
}

func (b *dbBuilder) sortedDirs() []string {
	dirs := make([]string, 0, len(b.entries))
	for d := range b.entries {
		dirs = append(dirs, d)
	}
	slices.Sort(dirs)
	return dirs
}

// epoch is the fixed member modification time. pacman reads members by name and
// ignores mtime, so stamping a constant makes the archive byte-stable across
// runs (repo-add stamps wall-clock time, so its archives are not reproducible).
var epoch = time.Unix(0, 0)

func writeTarDir(tw *tar.Writer, name string) error {
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Typeflag: tar.TypeDir,
		Mode:     0o755,
		ModTime:  epoch,
	}); err != nil {
		return fmt.Errorf("failed to write tar dir %s: %w", name, err)
	}
	return nil
}

func writeTarFile(tw *tar.Writer, name string, data []byte) error {
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(data)),
		ModTime:  epoch,
	}); err != nil {
		return fmt.Errorf("failed to write tar header %s: %w", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("failed to write %s: %w", name, err)
	}
	return nil
}

// splitEntryPath splits a db member path "<pkg>-<ver>/desc" into its directory
// and member name, returning ok=false for anything that is not a direct child of
// a package directory.
func splitEntryPath(p string) (dir, member string, ok bool) {
	p = strings.TrimSuffix(p, "/")
	i := strings.IndexByte(p, '/')
	if i <= 0 || i == len(p)-1 {
		return "", "", false
	}
	dir = p[:i]
	member = p[i+1:]
	if strings.Contains(member, "/") {
		return "", "", false
	}
	return dir, member, true
}

// descName extracts the %NAME% value from a raw desc entry. repo-add removes
// existing entries by package name, so the builder must recover it to dedupe.
func descName(desc []byte) string {
	lines := strings.Split(string(desc), "\n")
	for i, line := range lines {
		if line == "%NAME%" && i+1 < len(lines) {
			return lines[i+1]
		}
	}
	return ""
}

// filesEntry renders the "%FILES%" member: the header line followed by the
// sorted member list, exactly as repo-add writes it (no trailing blank line).
func filesEntry(files []string) []byte {
	var b strings.Builder
	b.WriteString("%FILES%\n")
	for _, f := range files {
		b.WriteString(f)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}
