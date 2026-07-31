package repository

import (
	"encoding/json"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

type logTokenRecord struct {
	JobID     string `json:"job_id"`
	ExpiresAt int64  `json:"expires_at"`
}

var spentLogTokenConsumption = consumptionPolicy{
	namespace:    kv.SpentLogTokens,
	emptyError:   "logtoken: empty token",
	errorContext: "logtoken: consume",
}

type logTokenRepository struct {
	kv kv.Store
}

func NewLogTokenRepository(store kv.Store) *logTokenRepository {
	return &logTokenRepository{kv: store}
}

func (r *logTokenRepository) StoreLogToken(token, jobID string, ttl time.Duration) error {
	if token == "" || jobID == "" {
		return errors.NewErr("logtoken: empty token or job id")
	}
	record, err := json.Marshal(logTokenRecord{JobID: jobID, ExpiresAt: time.Now().Add(ttl).Unix()})
	if err != nil {
		return errors.WrapErr(err, "logtoken: marshal")
	}
	if err := r.kv.Set(kv.LogTokens, token, record, ttl); err != nil {
		return errors.WrapErr(err, "logtoken: store")
	}
	return nil
}

func (r *logTokenRepository) ConsumeLogToken(token string) (string, bool, error) {
	if token == "" {
		return "", false, nil
	}
	v, ok, err := getOptional(r.kv, kv.LogTokens, token, "logtoken: get")
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
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
	_ = r.kv.Delete(kv.LogTokens, token)
	return record.JobID, true, nil
}
