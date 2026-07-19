package repository

import (
	"encoding/json"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/schema"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

type logTokenRecord struct {
	JobID     string `json:"job_id"`
	ExpiresAt int64  `json:"expires_at"`
}

var spentLogTokenConsumption = consumptionPolicy{
	namespace:    schema.SpentLogTokens,
	emptyError:   "logtoken: empty token",
	errorContext: "logtoken: consume",
}

// LogTokenRepository issues and redeems the one-time tokens that let a browser
// EventSource (which cannot send a bearer) open a job's build-log stream. A token
// is bound to one job and spent on first read, so a leaked stream URL cannot be
// reused.
type LogTokenRepository interface {
	// Mint issues a fresh token bound to jobID, valid for ttl.
	Mint(jobID string, ttl time.Duration) (string, error)
	// ConsumeLogToken returns the job id a token was bound to and deletes it, so a
	// second use finds nothing. A false ok means the token is unknown or spent.
	ConsumeLogToken(token string) (jobID string, ok bool, err error)
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
	tok, err := auth.NewOpaqueToken(32)
	if err != nil {
		return "", errors.WrapErr(err, "logtoken: mint")
	}
	record, err := json.Marshal(logTokenRecord{JobID: jobID, ExpiresAt: time.Now().Add(ttl).Unix()})
	if err != nil {
		return "", errors.WrapErr(err, "logtoken: marshal")
	}
	if err := r.kv.Set(schema.LogTokens, tok, record, ttl); err != nil {
		return "", errors.WrapErr(err, "logtoken: store")
	}
	return tok, nil
}

func (r *logTokenRepository) ConsumeLogToken(token string) (string, bool, error) {
	if token == "" {
		return "", false, nil
	}
	v, err := r.kv.Get(schema.LogTokens, token)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return "", false, nil
		}
		return "", false, errors.WrapErr(err, "logtoken: get")
	}
	record := logTokenRecord{}
	if err := json.Unmarshal(v, &record); err != nil {
		record.JobID = string(v)
		record.ExpiresAt = time.Now().Add(time.Minute).Unix()
	}
	ttl := time.Until(time.Unix(record.ExpiresAt, 0))
	if record.JobID == "" || ttl <= 0 {
		return "", false, nil
	}
	created, err := spentLogTokenConsumption.consume(r.kv, token, ttl)
	if err != nil {
		return "", false, err
	}
	if !created {
		return "", false, nil
	}
	_ = r.kv.Delete(schema.LogTokens, token)
	return record.JobID, true, nil
}
