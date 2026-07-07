package service

import (
	"context"

	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// WithPool attaches the pool collector for CollectPool's retention GC; nil (unset)
// disables the pool and makes CollectPool a no-op.
func (s *Service) WithPool(p repository.PoolCollector) *Service {
	s.pool = p
	return s
}

// CollectPool runs the pool retention GC under the configured policy, reclaiming
// unreferenced pool objects; safe to run online and a no-op when the pool is
// disabled.
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
