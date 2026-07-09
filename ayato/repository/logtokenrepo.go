package repository

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// logTokenNS holds one-time SSE log-stream tokens mapping a random token to the job
// id it grants, with a short TTL so an unused token self-evicts.
const logTokenNS = "logtoken"

// LogTokenRepository issues and redeems the one-time tokens that let a browser
// EventSource (which cannot send a bearer) open a job's build-log stream. A token
// is bound to one job and spent on first read, so a leaked stream URL cannot be
// reused.
type LogTokenRepository interface {
	// Mint issues a fresh token bound to jobID, valid for ttl.
	Mint(jobID string, ttl time.Duration) (string, error)
	// ConsumeLogToken returns the job id a token was bound to and deletes it, so a
	// second use finds nothing. A false ok means the token is unknown or spent.
	ConsumeLogToken(token string) (jobID string, ok bool)
}

type logTokenRepository struct {
	kv kv.Store
}

func NewLogTokenRepository(store kv.Store) LogTokenRepository {
	return &logTokenRepository{kv: store}
}

func (r *logTokenRepository) Mint(jobID string, ttl time.Duration) (string, error) {
	if jobID == "" {
		return "", errors.NewErr("logtoken: empty job id")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.WrapErr(err, "logtoken: read random")
	}
	tok := base64.RawURLEncoding.EncodeToString(b)
	if err := r.kv.Set(logTokenNS, tok, []byte(jobID), ttl); err != nil {
		return "", errors.WrapErr(err, "logtoken: store")
	}
	return tok, nil
}

func (r *logTokenRepository) ConsumeLogToken(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	v, err := r.kv.Get(logTokenNS, token)
	if err != nil {
		return "", false
	}
	// Delete-on-read: the token is single-use, so drop it before returning.
	_ = r.kv.Delete(logTokenNS, token)
	return string(v), true
}
