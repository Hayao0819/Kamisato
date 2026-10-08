package remote

import (
	"net/http"
	"time"
)

// DatabaseClient bounds repository metadata downloads. Commands own the client
// and pass it to the operation; repository services never select a global client.
func DatabaseClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }
