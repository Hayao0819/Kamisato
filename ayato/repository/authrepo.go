package repository

import (
	"strconv"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// allowNS namespaces the admin allowlist, the only persisted auth state; sessions, tokens, and OAuth state are all stateless-signed.
const allowNS = "allow"

type AllowedAdmin struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// AuthRepository persists the admin allowlist. Fail-closed: an empty allowlist,
// an unknown id, or a non-positive id all deny.
//
//go:generate mockgen -source=authrepo.go -destination=../test/mocks/authrepo.go -package=mocks
type AuthRepository interface {
	AddAdmin(id int64, login string) error
	RemoveAdmin(id int64) error
	IsAdmin(id int64) bool
	ListAdmins() ([]AllowedAdmin, error)
}

type authRepository struct {
	kv kv.Store
}

func NewAuthRepository(store kv.Store) AuthRepository {
	return &authRepository{kv: store}
}

func (r *authRepository) AddAdmin(id int64, login string) error {
	if id <= 0 {
		return errwrap.NewErr("auth: invalid github id")
	}
	return r.kv.Set(allowNS, strconv.FormatInt(id, 10), []byte(login), 0)
}

func (r *authRepository) RemoveAdmin(id int64) error {
	if id <= 0 {
		return errwrap.NewErr("auth: invalid github id")
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
		return nil, errwrap.WrapErr(err, "auth: list allowlist")
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
