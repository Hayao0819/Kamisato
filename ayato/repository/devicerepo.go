package repository

import (
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/schema"
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

var spentDeviceConsumption = consumptionPolicy{
	namespace:    schema.SpentDevices,
	emptyError:   "device: empty device code",
	errorContext: "device: consume",
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
	ConsumeDevice(deviceCode string) (consumed bool, err error)
}

type deviceRepository struct {
	kv kv.Store
}

func NewDeviceRepository(store kv.Store) DeviceRepository {
	return &deviceRepository{kv: store}
}

func (r *deviceRepository) CreateDevice(deviceCode, userCode string, ttl time.Duration) error {
	if deviceCode == "" || userCode == "" {
		return errors.NewErr("device: empty device or user code")
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
	if err := r.kv.Set(schema.DeviceUserIndex, userCode, []byte(deviceCode), ttl); err != nil {
		return errors.WrapErr(err, "device: index user code")
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

func (r *deviceRepository) ConsumeDevice(deviceCode string) (bool, error) {
	rec, ok, err := r.getByCode(deviceCode)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	ttl := time.Until(time.Unix(rec.ExpiresAt, 0))
	if ttl <= 0 {
		return false, nil
	}
	created, err := spentDeviceConsumption.consume(r.kv, deviceCode, ttl)
	if err != nil {
		return false, err
	}
	if !created {
		return false, nil
	}
	_ = r.kv.Delete(schema.DeviceUserIndex, rec.UserCode)
	_ = r.kv.Delete(schema.Devices, deviceCode)
	return true, nil
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
