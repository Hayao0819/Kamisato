package repository

import (
	"encoding/json"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv/schema"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func (r *deviceRepository) putByCode(
	deviceCode string,
	record deviceRecord,
	ttl time.Duration,
) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return errors.WrapErr(err, "device: marshal record")
	}
	if err := r.kv.Set(schema.Devices, deviceCode, raw, ttl); err != nil {
		return errors.WrapErr(err, "device: store record")
	}
	return nil
}

func (r *deviceRepository) getByCode(deviceCode string) (deviceRecord, bool, error) {
	if deviceCode == "" {
		return deviceRecord{}, false, nil
	}
	raw, ok, err := getOptional(r.kv, schema.Devices, deviceCode, "device: get record")
	if err != nil || !ok {
		return deviceRecord{}, ok, err
	}
	var record deviceRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return deviceRecord{}, false, errors.WrapErr(err, "device: unmarshal record")
	}
	// ExpiresAt remains authoritative for backends with coarse TTL granularity.
	if time.Now().After(time.Unix(record.ExpiresAt, 0)) {
		return deviceRecord{}, false, nil
	}
	return record, true, nil
}

func (r *deviceRepository) getByUserCode(userCode string) (deviceRecord, bool, error) {
	if userCode == "" {
		return deviceRecord{}, false, nil
	}
	code, ok, err := getOptional(
		r.kv,
		schema.DeviceUserIndex,
		userCode,
		"device: get user index",
	)
	if err != nil || !ok {
		return deviceRecord{}, ok, err
	}
	return r.getByCode(string(code))
}
