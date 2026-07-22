package shared

import (
	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/serverstore"
)

// AyatoClient returns an authenticated Ayato client.
func AyatoClient(srv *AyatoServer) (*client.Ayato, error) {
	return serverstore.NewClient(srv)
}
