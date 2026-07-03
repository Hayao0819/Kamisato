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
