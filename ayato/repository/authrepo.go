package repository

import (
	"strconv"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

// allowNS is the kv namespace holding the admin allowlist: one entry per
// allowed GitHub id (key = decimal id, value = login label). It is the ONLY
// persisted auth state — sessions, CLI tokens, one-time codes, and OAuth state
// are all stateless-signed.
const allowNS = "allow"

// AllowedAdmin pairs a GitHub id with its stored login label.
type AllowedAdmin struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// AuthRepository persists the auth domain's server-side state. Today that is the
// admin allowlist; the interface is named for the domain (not the allowlist) so
// future auth state can join without a rename. Everything is fail-closed: an
// empty allowlist denies, unknown ids are rejected, and any non-positive id is
// refused.
//
//go:generate mockgen -source=authrepo.go -destination=../test/mocks/authrepo.go -package=mocks
type AuthRepository interface {
	// AddAdmin inserts (or updates) an allowed GitHub id with an optional login label.
	AddAdmin(id int64, login string) error
	// RemoveAdmin deletes a GitHub id from the allowlist.
	RemoveAdmin(id int64) error
	// IsAdmin reports whether id is on the allowlist. Fail-closed: a non-positive
	// id or any read miss returns false, so an empty allowlist denies everyone.
	IsAdmin(id int64) bool
	// ListAdmins returns every allowlisted GitHub id with its login label.
	ListAdmins() ([]AllowedAdmin, error)
}

// authRepository is the admin allowlist on the shared kv.Store, namespaced under
// allowNS. It shares the one kv built by New with the package-metadata store.
type authRepository struct {
	kv kv.Store
}

// NewAuthRepository returns an AuthRepository backed by the shared kv store.
func NewAuthRepository(store kv.Store) AuthRepository {
	return &authRepository{kv: store}
}

func (r *authRepository) AddAdmin(id int64, login string) error {
	if id <= 0 {
		return utils.NewErr("auth: invalid github id")
	}
	return r.kv.Set(allowNS, strconv.FormatInt(id, 10), []byte(login), 0)
}

func (r *authRepository) RemoveAdmin(id int64) error {
	if id <= 0 {
		return utils.NewErr("auth: invalid github id")
	}
	return r.kv.Delete(allowNS, strconv.FormatInt(id, 10))
}

func (r *authRepository) IsAdmin(id int64) bool {
	if id <= 0 {
		return false
	}
	_, err := r.kv.Get(allowNS, strconv.FormatInt(id, 10))
	return err == nil
}

func (r *authRepository) ListAdmins() ([]AllowedAdmin, error) {
	entries, err := r.kv.List(allowNS)
	if err != nil {
		return nil, utils.WrapErr(err, "auth: list allowlist")
	}
	var out []AllowedAdmin
	for _, e := range entries {
		id, perr := strconv.ParseInt(e.Key, 10, 64)
		if perr != nil {
			continue
		}
		out = append(out, AllowedAdmin{ID: id, Login: string(e.Value)})
	}
	return out, nil
}
