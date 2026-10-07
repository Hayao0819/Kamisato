package cli

import (
	"github.com/Hayao0819/Kamisato/internal/api/ayato"
	ayatostore "github.com/Hayao0819/Kamisato/internal/api/ayato/auth/store"
)

// AyatoClient returns an authenticated Ayato client.
func AyatoClient(srv *AyatoServer) (*ayato.Client, error) {
	return ayatostore.NewStoredClient(srv)
}
