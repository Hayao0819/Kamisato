package migrate

import (
	"context"
	"fmt"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func init() { register(unpool{}) }

// unpool (layout 1) moves package bytes out of the content-addressed pool into the
// direct repo/arch/filename layout. These are the historical pool constants (the
// pool package itself is gone).
const (
	poolObjectPrefix = "_pool_/objects/"
)

type unpool struct{}

func (unpool) Version() int { return 1 }
func (unpool) Name() string { return "unpool" }

// Expand copies each pooled object to the repo/arch/filename its pointer names. The
// pool still serves until Contract, so this is safe under live traffic.
func (unpool) Expand(ctx context.Context, s *Stores) error {
	mover, err := s.ObjectMover()
	if err != nil {
		return err
	}
	ptrs, err := s.KV.List(kv.LegacyPoolPointers)
	if err != nil {
		return errors.WrapErr(err, "list pool pointers")
	}
	for _, e := range ptrs {
		if err := ctx.Err(); err != nil {
			return err
		}
		hash := string(e.Value)
		if hash == "" {
			continue
		}
		if err := mover.CopyObject(poolObjectPrefix+hash, e.Key); err != nil {
			return errors.WrapErr(err, fmt.Sprintf("copy %s -> %s", hash, e.Key))
		}
	}
	return nil
}

// Contract deletes the pool objects and its KV indices. pkgfile is dropped too; it
// rebuilds lazily from the .db under the new key scheme.
func (unpool) Contract(ctx context.Context, s *Stores) error {
	mover, err := s.ObjectMover()
	if err != nil {
		return err
	}
	objs, err := mover.ListObjects(poolObjectPrefix)
	if err != nil {
		return errors.WrapErr(err, "list pool objects")
	}
	for _, k := range objs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := mover.DeleteObject(k); err != nil {
			return errors.WrapErr(err, "delete pool object")
		}
	}
	for _, ns := range []string{kv.LegacyPoolPointers, kv.LegacyPoolObjects, kv.PackageFiles} {
		entries, err := s.KV.List(ns)
		if err != nil {
			return errors.WrapErr(err, "list "+ns)
		}
		keys := make([]string, len(entries))
		for i, e := range entries {
			keys[i] = e.Key
		}
		if err := BulkDelete(s.KV, ns, keys); err != nil {
			return errors.WrapErr(err, "delete "+ns)
		}
	}
	return nil
}
