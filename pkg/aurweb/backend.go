package aurweb

import "context"

// SuggestLimit caps the number of name completions a suggest query returns.
const SuggestLimit = 20

// Backend is the package set a host manages; the Server falls back to Upstream for names the backend does not return.
type Backend interface {
	// Info returns full records for pkgnames the backend manages; unmanaged names are omitted.
	Info(ctx context.Context, names []string) ([]Pkg, error)

	// Search returns base-level records matching arg under the given field.
	Search(ctx context.Context, by By, arg string) ([]Pkg, error)

	// Suggest returns up to SuggestLimit pkgnames (or pkgbases when pkgbase is
	// true) that begin with arg.
	Suggest(ctx context.Context, arg string, pkgbase bool) ([]string, error)

	// All returns every package the backend manages, for the bulk dump endpoints.
	All(ctx context.Context) ([]Pkg, error)

	// SourceURL returns the clone redirect target for a pkgbase; ok=false when the backend does not manage it
	// (Server falls through to Upstream git base), preserving any trailing git path.
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
