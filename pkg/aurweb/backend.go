package aurweb

import "context"

// Backend is the package set a host manages. The Server queries it first and,
// when an Upstream is configured, falls back to upstream for anything the
// backend does not return.
type Backend interface {
	// Info returns full (info-level) records for the given pkgnames that the
	// backend manages. Names it does not manage are simply omitted; order is
	// not significant.
	Info(ctx context.Context, names []string) ([]Pkg, error)

	// Search returns base-level records matching arg under the given field.
	Search(ctx context.Context, by By, arg string) ([]Pkg, error)

	// Suggest returns up to 20 pkgnames (or pkgbases when pkgbase is true) that
	// begin with arg.
	Suggest(ctx context.Context, arg string, pkgbase bool) ([]string, error)

	// All returns every package the backend manages, for the bulk dump endpoints.
	All(ctx context.Context) ([]Pkg, error)

	// SourceURL returns the git clone base URL for a pkgbase: a client cloning
	// "<host>/<pkgbase>.git" is redirected to "<target>" with the trailing git
	// path preserved. ok is false when the backend does not manage the pkgbase,
	// in which case the Server falls through to the Upstream git base.
	SourceURL(ctx context.Context, pkgbase string) (target string, ok bool, err error)
}

// Upstream is an optional fallback (the real AUR) for packages a Backend does
// not manage. A nil Upstream makes the Server a closed, private instance.
type Upstream interface {
	Info(ctx context.Context, names []string) ([]Pkg, error)
	Search(ctx context.Context, by By, arg string) ([]Pkg, error)
	Suggest(ctx context.Context, arg string, pkgbase bool) ([]string, error)

	// GitBase is the base whose "<GitBase>/<pkgbase>.git" satisfies a git clone,
	// e.g. "https://aur.archlinux.org".
	GitBase() string

	// SnapshotURL returns the upstream cgit snapshot tarball URL for a pkgbase.
	SnapshotURL(pkgbase string) string

	// PlainURL returns the upstream cgit raw-PKGBUILD URL for a pkgname.
	PlainURL(pkgname string) string
}
