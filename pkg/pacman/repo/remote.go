package repo

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/compress"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// dbHTTPClient downloads repo databases with a bounded timeout so a slow or
// hung mirror can't block the caller indefinitely.
var dbHTTPClient = &http.Client{Timeout: 30 * time.Second}

// ErrRepoNotFound reports that the remote db is absent (HTTP 404): the repo is
// not configured on the server, or the URL/arch is wrong. It is distinct from a
// transport failure or a 5xx/auth error so callers can tell a fix-your-config
// problem from an unreachable server, and never mistake either for an empty repo.
var ErrRepoNotFound = errors.New("remote repository not found")

// ErrUnsupportedDBFormat reports a database this parser cannot read: a CachyOS
// `repo-add --use-new-db-format` archive, which embeds a single SQLite pacman.db
// in place of the per-package text desc/files entries every other Arch repo
// ships. Detecting it turns a silently-empty repo into a clear failure; the flag
// is off by default in CachyOS's repo-add and unused by their published mirrors.
var ErrUnsupportedDBFormat = errors.New("unsupported repository database format")

type RemoteRepo struct {
	Name   string
	Server string
	Pkgs   []*pkg.BinaryPackage
}

func (r *RemoteRepo) PkgByPkgName(pkgname string) *pkg.BinaryPackage {
	for _, p := range r.Pkgs {
		if p.Name() == pkgname {
			return p
		}
	}
	return nil
}

func (r *RemoteRepo) PkgByPkgBase(pkgbase string) *pkg.BinaryPackage {
	for _, p := range r.Pkgs {
		if p.Base() == pkgbase {
			return p
		}
	}
	return nil
}

func RepoFromURL(server string, name string) (*RemoteRepo, error) {
	dburl, err := url.JoinPath(server, name+".db")
	if err != nil {
		return nil, err
	}

	resp, err := dbHTTPClient.Get(dburl)
	if err != nil {
		return nil, fmt.Errorf("failed to download database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, dburl)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status while downloading: %s", resp.Status)
	}

	r, err := RemoteRepoFromDB(name, resp.Body)
	if r != nil {
		r.Server = server
	}
	return r, err
}

func RepoFromDBFile(name string, dbfile string) (*RemoteRepo, error) {
	db, err := os.Open(dbfile)
	if err != nil {
		return nil, fmt.Errorf("failed to open database file: %w", err)
	}
	defer db.Close()

	return RemoteRepoFromDB(name, db)
}

// RemoteRepoFromDB parses a repository .db stream (a compressed tar of
// "<pkg>-<ver>/desc" entries) into a RemoteRepo.
func RemoteRepoFromDB(name string, db io.Reader) (*RemoteRepo, error) {
	gzr, _, err := compress.DetectCompression(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create decompressor: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	pkgs := make([]*pkg.BinaryPackage, 0)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read db tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeDir || !strings.HasSuffix(hdr.Name, "/desc") {
			if isNewDBFormat(hdr, tr) {
				return nil, fmt.Errorf("%w: %q is a SQLite pacman.db", ErrUnsupportedDBFormat, hdr.Name)
			}
			continue
		}

		var buf strings.Builder
		if _, err := io.Copy(&buf, tr); err != nil { //nolint:gosec // db is operator-controlled
			return nil, fmt.Errorf("failed to read %s: %w", hdr.Name, err)
		}
		desc, err := raiou.ParseDescString(buf.String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", hdr.Name, err)
		}
		info, err := desc.ToPKGINFO()
		if err != nil {
			return nil, fmt.Errorf("failed to convert %s: %w", hdr.Name, err)
		}
		pkgs = append(pkgs, pkg.NewBinaryPackage(desc.FileName, info))
	}
	return &RemoteRepo{Name: name, Pkgs: pkgs}, nil
}

// newDBFormatMagic is the leading signature of a SQLite 3 file. CachyOS's Rust
// repo-add, run with --use-new-db-format, stores the whole repository as one
// embedded SQLite pacman.db instead of per-package desc entries; that file
// begins with this magic.
const newDBFormatMagic = "SQLite format 3\x00"

// isNewDBFormat reports whether a non-desc tar member is the embedded SQLite
// database of a CachyOS new-db-format repository. It sniffs the SQLite magic so
// detection does not hinge on the member's exact name.
func isNewDBFormat(hdr *tar.Header, r io.Reader) bool {
	if hdr.Typeflag != tar.TypeReg {
		return false
	}
	var magic [len(newDBFormatMagic)]byte
	n, _ := io.ReadFull(r, magic[:])
	return n == len(magic) && string(magic[:]) == newDBFormatMagic
}
