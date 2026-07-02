package pkg

import (
	"archive/tar"
	"debug/elf"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// SonamesOf returns the DT_SONAME values of the shared objects a package ships,
// deduplicated and sorted. path may be a built package archive (.pkg.tar.*), an
// already-extracted directory, or a single shared object. Files without a
// DT_SONAME (executables, data) are ignored, and a file that fails to parse as
// ELF is skipped rather than failing the whole scan, so a stray non-ELF ".so"
// name cannot break detection.
func SonamesOf(pathOrDir string) ([]string, error) {
	fi, err := os.Stat(pathOrDir)
	if err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	if fi.IsDir() {
		err = sonamesFromDir(pathOrDir, set)
	} else {
		err = sonamesFromFile(pathOrDir, set)
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	slices.Sort(out)
	return out, nil
}

func sonamesFromDir(dir string, set map[string]struct{}) error {
	return filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() || !isSharedObjectName(d.Name()) {
			return nil
		}
		f, oerr := elf.Open(p)
		if oerr != nil {
			return nil // not a valid ELF; skip
		}
		defer func() { _ = f.Close() }()
		addSonames(f, set)
		return nil
	})
}

// sonamesFromFile reads a single shared object directly when the path is itself
// an ELF, otherwise treats it as a package archive.
func sonamesFromFile(p string, set map[string]struct{}) error {
	if ef, err := elf.Open(p); err == nil {
		defer func() { _ = ef.Close() }()
		addSonames(ef, set)
		return nil
	}
	return sonamesFromArchive(p, set)
}

func sonamesFromArchive(archive string, set map[string]struct{}) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	return walkPackageTar(f, func(hdr *tar.Header, content io.Reader) (bool, error) {
		if hdr.Typeflag != tar.TypeReg || !isSharedObjectName(path.Base(hdr.Name)) {
			return false, nil
		}
		if err := sonameFromStream(content, set); err != nil {
			return false, err
		}
		return false, nil
	})
}

// sonameFromStream spills one shared object to a temp file so debug/elf gets the
// io.ReaderAt it needs, without buffering a possibly large library in memory.
func sonameFromStream(r io.Reader, set map[string]struct{}) error {
	tmp, err := os.CreateTemp("", "miko-so-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	defer func() { _ = tmp.Close() }()

	if _, err := io.Copy(tmp, r); err != nil { //nolint:gosec // tar member is a built package we produced
		return err
	}
	ef, err := elf.Open(tmp.Name())
	if err != nil {
		return nil // not a valid ELF; skip
	}
	defer func() { _ = ef.Close() }()
	addSonames(ef, set)
	return nil
}

func addSonames(f *elf.File, set map[string]struct{}) {
	names, err := f.DynString(elf.DT_SONAME)
	if err != nil {
		return // no dynamic section / not a shared object
	}
	for _, n := range names {
		if n != "" {
			set[n] = struct{}{}
		}
	}
}

// isSharedObjectName matches the pacman shared-object naming: a bare "libfoo.so"
// or a versioned "libfoo.so.1[.2.3]". It filters the tar/dir walk so only real
// candidates are opened as ELF.
func isSharedObjectName(name string) bool {
	return strings.HasSuffix(name, ".so") || strings.Contains(name, ".so.")
}

// SonameBump records a shared object whose provided soname changed between two
// builds. New is empty when the soname disappeared entirely.
type SonameBump struct {
	Base string // unversioned base, e.g. "libfoo.so"
	Old  string // previous soname, e.g. "libfoo.so.1"
	New  string // current soname, e.g. "libfoo.so.2"; empty means removed
}

// DetectBumps compares the sonames a package provided at its last build against
// the current build and returns the bumps. A bump is a base whose versioned
// soname changed (libfoo.so.1 -> libfoo.so.2) or that is no longer provided; a
// newly added soname is not a bump, since nothing depended on it yet.
func DetectBumps(oldSonames, newSonames []string) []SonameBump {
	oldByBase := indexByBase(oldSonames)
	newByBase := indexByBase(newSonames)

	var bumps []SonameBump
	for base, oldName := range oldByBase {
		newName, ok := newByBase[base]
		switch {
		case !ok:
			bumps = append(bumps, SonameBump{Base: base, Old: oldName})
		case oldName != newName:
			bumps = append(bumps, SonameBump{Base: base, Old: oldName, New: newName})
		}
	}
	slices.SortFunc(bumps, func(a, b SonameBump) int { return strings.Compare(a.Base, b.Base) })
	return bumps
}

func indexByBase(sonames []string) map[string]string {
	m := make(map[string]string, len(sonames))
	for _, s := range sonames {
		m[sonameBase(s)] = s
	}
	return m
}

// sonameBase strips the version suffix, returning the part through ".so". A
// soname carries its major version after ".so." (libpython3.11.so.1.0 -> the
// "3.11" is part of the name, the "1.0" is the version), so split on the first
// ".so." rather than the first dot.
func sonameBase(s string) string {
	if i := strings.Index(s, ".so."); i >= 0 {
		return s[:i+len(".so")]
	}
	return s
}
