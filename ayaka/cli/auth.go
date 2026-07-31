package cli

import (
	"github.com/Hayao0819/Kamisato/internal/ayatoapi"
)

// AyatoClient returns an authenticated Ayato client.
func AyatoClient(srv *AyatoServer) (*ayatoapi.Ayato, error) {
	return ayatoapi.NewStoredClient(srv)
}
