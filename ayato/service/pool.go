package service

import (
	"context"

	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// WithPool attaches the content-addressed pool collector so CollectPool can run
// the retention GC. Unset (nil) means the pool is disabled and CollectPool is a
// no-op.
func (s *Service) WithPool(p repository.PoolCollector) *Service {
	s.pool = p
	return s
}

// CollectPool runs the pool retention GC with the deployment's configured policy:
// it reclaims pool objects no repo points at, keeping the newest
// pool.keep_unreferenced versions per pkgbase and anything unreferenced for less
// than pool.retention_window_hours. It is safe to run online and a no-op when the
// pool is disabled.
func (s *Service) CollectPool(ctx context.Context) (repository.PoolGCResult, error) {
	if s.pool == nil {
		return repository.PoolGCResult{}, nil
	}
	policy := repository.PoolPolicy{
		KeepUnreferenced: s.cfg.Pool.KeepUnreferenced,
		RetentionWindow:  s.cfg.Pool.RetentionWindow(),
	}
	res, err := s.pool.CollectPool(ctx, policy)
	if err != nil {
		return res, errwrap.WrapErr(err, "collect pool")
	}
	return res, nil
}
