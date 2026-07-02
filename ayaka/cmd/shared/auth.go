package shared

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/buildclient"
)

// WithServerAuth runs op with srv's stored access token and, when ayato reports the
// token merely expired, transparently trades the stored refresh token for a fresh
// one, persists the rotated pair, and retries op once. A missing or failed refresh
// surfaces as op's error (or a clear re-login message). This is the single wrapper
// every authenticated ayaka command uses so short-lived access tokens stay
// invisible to the user.
func WithServerAuth(ctx context.Context, srv *AyatoServer, op func(ctx context.Context, token string) error) error {
	refresh := blinkyutils.LoadRefreshSecret(srv.URL)
	persist := func(access, newRefresh string) error {
		return saveServerTokens(srv.URL, srv.Username, access, newRefresh)
	}
	return buildclient.WithRefresh(ctx, srv.URL, srv.Password, refresh, persist, op)
}

// saveServerTokens persists a rotated access+refresh pair for an already-registered
// server, mirroring the login path (keyring-first with a file-DB fallback for the
// access token; keyring-only for the refresh token).
func saveServerTokens(server, username, access, refresh string) error {
	db, err := blinkyutils.ReadServerDB()
	if err != nil {
		return err
	}
	entry := db.Servers[server]
	if username != "" {
		entry.Username = username
	}
	if blinkyutils.StoreSecret(server, access) {
		entry.Password = ""
	} else {
		entry.Password = access
	}
	blinkyutils.StoreRefreshSecret(server, refresh)
	db.Servers[server] = entry
	return blinkyutils.SaveServerDB(db)
}
