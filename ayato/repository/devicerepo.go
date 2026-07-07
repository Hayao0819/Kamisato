package repository

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// deviceNS maps a device_code to its record; deviceUserNS indexes the user_code to
// its device_code so the approval page can find the pending record. Both carry the
// authorization-window TTL (RFC 8628) so unredeemed requests self-evict.
const (
	deviceNS     = "device"
	deviceUserNS = "deviceuc"
)

// deviceRecord is the kv-persisted device authorization. ExpiresAt is stored because
// kv exposes no read-back of residual TTL, needed to re-set the entry on a status
// transition.
type deviceRecord struct {
	DeviceCode string `json:"device_code"`
	UserCode   string `json:"user_code"`
	Status     string `json:"status"`
	GitHubID   int64  `json:"github_id,omitempty"`
	Login      string `json:"login,omitempty"`
	ExpiresAt  int64  `json:"expires_at"`
}

// DeviceRepository stores RFC 8628 device-authorization requests in the shared kv
// so the polling client and the separate approval browser can rendezvous without
// ayato holding process-local state.
type DeviceRepository interface {
	// CreateDevice records a fresh pending authorization for ttl.
	CreateDevice(deviceCode, userCode string, ttl time.Duration) error
	// LookupByUserCode returns the status of the live record a user_code maps to;
	// ok is false when none exists (unknown or expired code).
	LookupByUserCode(userCode string) (status string, ok bool, err error)
	// ApproveDevice attaches the authenticated identity and marks the record
	// approved; ok is false when no live record matches.
	ApproveDevice(userCode string, githubID int64, login string) (ok bool, err error)
	// DenyDevice marks the record denied (user rejected or not allowlisted).
	DenyDevice(userCode string) (ok bool, err error)
	// PollDevice returns the current state a device_code polls for; ok is false
	// when the record is absent or expired.
	PollDevice(deviceCode string) (status string, githubID int64, login string, ok bool, err error)
	// ConsumeDevice removes an authorization once its token has been issued so it
	// cannot be redeemed twice.
	ConsumeDevice(deviceCode string) error
}

type deviceRepository struct {
	kv kv.Store
}

func NewDeviceRepository(store kv.Store) DeviceRepository {
	return &deviceRepository{kv: store}
}

func (r *deviceRepository) CreateDevice(deviceCode, userCode string, ttl time.Duration) error {
	if deviceCode == "" || userCode == "" {
		return errwrap.NewErr("device: empty device or user code")
	}
	rec := deviceRecord{
		DeviceCode: deviceCode,
		UserCode:   userCode,
		Status:     auth.DevicePending,
		ExpiresAt:  time.Now().Add(ttl).Unix(),
	}
	if err := r.putByCode(deviceCode, rec, ttl); err != nil {
		return err
	}
	if err := r.kv.Set(deviceUserNS, userCode, []byte(deviceCode), ttl); err != nil {
		return errwrap.WrapErr(err, "device: index user code")
	}
	return nil
}

func (r *deviceRepository) LookupByUserCode(userCode string) (string, bool, error) {
	rec, ok, err := r.getByUserCode(userCode)
	if err != nil || !ok {
		return "", ok, err
	}
	return rec.Status, true, nil
}

func (r *deviceRepository) ApproveDevice(userCode string, githubID int64, login string) (bool, error) {
	return r.transition(userCode, func(rec *deviceRecord) {
		rec.Status = auth.DeviceApproved
		rec.GitHubID = githubID
		rec.Login = login
	})
}

func (r *deviceRepository) DenyDevice(userCode string) (bool, error) {
	return r.transition(userCode, func(rec *deviceRecord) { rec.Status = auth.DeviceDenied })
}

func (r *deviceRepository) PollDevice(deviceCode string) (string, int64, string, bool, error) {
	rec, ok, err := r.getByCode(deviceCode)
	if err != nil || !ok {
		return "", 0, "", ok, err
	}
	return rec.Status, rec.GitHubID, rec.Login, true, nil
}

func (r *deviceRepository) ConsumeDevice(deviceCode string) error {
	rec, ok, err := r.getByCode(deviceCode)
	if err != nil {
		return err
	}
	if ok {
		_ = r.kv.Delete(deviceUserNS, rec.UserCode)
	}
	return r.kv.Delete(deviceNS, deviceCode)
}

// transition re-stores the user_code's record after mutate; ok is false when the
// record is absent or expired.
func (r *deviceRepository) transition(userCode string, mutate func(*deviceRecord)) (bool, error) {
	rec, ok, err := r.getByUserCode(userCode)
	if err != nil || !ok {
		return ok, err
	}
	ttl := time.Until(time.Unix(rec.ExpiresAt, 0))
	if ttl <= 0 {
		return false, nil
	}
	mutate(&rec)
	if err := r.putByCode(rec.DeviceCode, rec, ttl); err != nil {
		return false, err
	}
	return true, nil
}

func (r *deviceRepository) putByCode(deviceCode string, rec deviceRecord, ttl time.Duration) error {
	raw, err := json.Marshal(rec)
	if err != nil {
		return errwrap.WrapErr(err, "device: marshal record")
	}
	if err := r.kv.Set(deviceNS, deviceCode, raw, ttl); err != nil {
		return errwrap.WrapErr(err, "device: store record")
	}
	return nil
}

func (r *deviceRepository) getByCode(deviceCode string) (deviceRecord, bool, error) {
	if deviceCode == "" {
		return deviceRecord{}, false, nil
	}
	raw, err := r.kv.Get(deviceNS, deviceCode)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return deviceRecord{}, false, nil
		}
		return deviceRecord{}, false, errwrap.WrapErr(err, "device: get record")
	}
	var rec deviceRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return deviceRecord{}, false, errwrap.WrapErr(err, "device: unmarshal record")
	}
	// Guard against a backend with coarse TTL granularity serving a just-expired
	// entry: the absolute ExpiresAt is authoritative.
	if time.Now().After(time.Unix(rec.ExpiresAt, 0)) {
		return deviceRecord{}, false, nil
	}
	return rec, true, nil
}

func (r *deviceRepository) getByUserCode(userCode string) (deviceRecord, bool, error) {
	if userCode == "" {
		return deviceRecord{}, false, nil
	}
	code, err := r.kv.Get(deviceUserNS, userCode)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return deviceRecord{}, false, nil
		}
		return deviceRecord{}, false, errwrap.WrapErr(err, "device: get user index")
	}
	return r.getByCode(string(code))
}
