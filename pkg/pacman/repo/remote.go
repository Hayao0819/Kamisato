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

// dbHTTPClient is an HTTP client with a bounded timeout to prevent a hung mirror from blocking indefinitely.
var dbHTTPClient = &http.Client{Timeout: 30 * time.Second}

// ErrRepoNotFound is an HTTP 404: the repo is absent (misconfigured URL/arch), distinct from transport failures so callers can tell a config error from an unreachable server.
var ErrRepoNotFound = errors.New("remote repository not found")

// ErrUnsupportedDBFormat is a CachyOS `--use-new-db-format` archive (SQLite instead of text desc/files);
// detecting it turns a silently-empty repo into a clear failure. Off by default in CachyOS repo-add and unused by their published mirrors.
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

// RemoteRepoFromDB parses a .db stream (compressed tar of <pkg>-<ver>/desc entries) into a RemoteRepo.
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

// newDBFormatMagic is the SQLite 3 signature; CachyOS's --use-new-db-format stores the repo as a single embedded SQLite pacman.db beginning with this magic.
const newDBFormatMagic = "SQLite format 3\x00"

// isNewDBFormat reports whether a tar member is CachyOS's embedded SQLite DB by sniffing the magic, not the member name.
func isNewDBFormat(hdr *tar.Header, r io.Reader) bool {
	if hdr.Typeflag != tar.TypeReg {
		return false
	}
	var magic [len(newDBFormatMagic)]byte
	n, _ := io.ReadFull(r, magic[:])
	return n == len(magic) && string(magic[:]) == newDBFormatMagic
}
