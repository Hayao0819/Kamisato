package auth

import (
	"strconv"

	"github.com/Hayao0819/Kamisato/ayato/kv"
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

// AllowlistRepo is the admin allowlist on the shared kv.Store. Everything is
// fail-closed: an empty allowlist denies, unknown ids are rejected, and any
// non-positive id is refused.
type AllowlistRepo struct {
	kv kv.Store
}

// NewAllowlistRepo returns an allowlist backed by the shared kv store.
func NewAllowlistRepo(store kv.Store) *AllowlistRepo {
	return &AllowlistRepo{kv: store}
}

// Add inserts (or updates) an allowed GitHub id with an optional login label.
func (r *AllowlistRepo) Add(id int64, login string) error {
	if id <= 0 {
		return utils.NewErr("auth: invalid github id")
	}
	return r.kv.Set(allowNS, strconv.FormatInt(id, 10), []byte(login), 0)
}

// Remove deletes a GitHub id from the allowlist.
func (r *AllowlistRepo) Remove(id int64) error {
	if id <= 0 {
		return utils.NewErr("auth: invalid github id")
	}
	return r.kv.Delete(allowNS, strconv.FormatInt(id, 10))
}

// Has reports whether id is on the allowlist. Fail-closed: a non-positive id or
// any read miss returns false. An empty allowlist therefore denies everyone.
func (r *AllowlistRepo) Has(id int64) bool {
	if id <= 0 {
		return false
	}
	_, err := r.kv.Get(allowNS, strconv.FormatInt(id, 10))
	return err == nil
}

// ListAllowed returns every allowlisted GitHub id with its login label.
func (r *AllowlistRepo) ListAllowed() ([]AllowedAdmin, error) {
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

// SeedBootstrap adds id to the allowlist when id > 0 and the allowlist is empty,
// matching the previous on-disk bootstrap behavior. A bootstrapID <= 0 is
// ignored (no seed), leaving the allowlist empty (fail-closed: denies all).
func SeedBootstrap(repo *AllowlistRepo, id int64) error {
	if id <= 0 {
		return nil
	}
	admins, err := repo.ListAllowed()
	if err != nil {
		return utils.WrapErr(err, "auth: list allowlist for seed")
	}
	if len(admins) == 0 {
		if err := repo.Add(id, ""); err != nil {
			return utils.WrapErr(err, "auth: seed bootstrap admin")
		}
	}
	return nil
}
