package shared

import (
	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/serverstore"
)

// AyatoClient returns an authenticated Ayato client.
func AyatoClient(srv *AyatoServer) (*client.Ayato, error) {
	source := serverstore.NewTokenSource(&serverstore.Endpoint{
		URL:         srv.URL,
		Username:    srv.Username,
		AccessToken: srv.AccessToken,
	})
	return client.NewAyato(srv.URL, source)
}
