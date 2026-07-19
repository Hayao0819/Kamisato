// Package buildclient preserves the legacy function-based client API.
package buildclient

import (
	"github.com/Hayao0819/Kamisato/internal/client"
)

var ErrAccessTokenExpired = client.ErrAccessTokenExpired

func ayato(base, token string) (*client.Ayato, error) {
	return client.NewAyato(base, client.StaticBearer(token))
}
