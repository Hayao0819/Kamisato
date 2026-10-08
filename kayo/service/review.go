package service

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/kayo/audit"
	"github.com/Hayao0819/Kamisato/kayo/audit/llm"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/gitserve"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

type ReviewOptions struct {
	Ref     string
	Approve bool
	Force   bool
}

type ReviewResult struct {
	Resolved    Resolved
	Report      audit.Report
	Verdict     trust.Verdict
	Advisory    *llm.Advisory
	AdvisoryErr error // an advisory is never a trust gate
	Approved    bool
}

// Review audits a target and optionally pins/approves exactly the reviewed
// commit. It owns the temporary checkout; callers receive decisions, not paths
// whose lifetime they have to manage to complete the approval.
func Review(ctx context.Context, cfg *kayoconfig.KayoConfig, target string, options ReviewOptions, advisory func(context.Context, string) (*llm.Advisory, error)) (*ReviewResult, error) {
	resolved, cleanup, err := resolveForReview(ctx, cfg, target, options.Ref, options.Approve)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	report, err := audit.Scan(resolved.Dir)
	if err != nil {
		return nil, err
	}
	store, err := trust.Open(cfg.ResolvedTrustStore())
	if err != nil {
		return nil, err
	}
	result := &ReviewResult{
		Resolved: resolved,
		Report:   report,
		Verdict:  store.Evaluate(resolved.Source, resolved.Pkgbase, resolved.Maintainer),
	}
	if advisory != nil {
		result.Advisory, result.AdvisoryErr = advisory(ctx, resolved.Dir)
	}
	if !options.Approve {
		if report.Max() >= audit.SevHigh {
			return result, errors.NewErr("audit found high-severity issues")
		}
		return result, nil
	}
	if report.Max() >= audit.SevHigh && !options.Force {
		return result, errors.NewErr("refusing to trust: high-severity findings (use --force to override)")
	}
	if err := resolved.RequirePinnedCommit(); err != nil {
		return result, err
	}
	if err := gitserve.Materialize(ctx, cfg.ServedRoot(), resolved.Pkgbase, resolved.Dir, resolved.Commit); err != nil {
		return result, errors.WrapErr(err, "failed to pin reviewed commit")
	}
	store.Approve(trust.Approval{
		Pkgbase:    resolved.Pkgbase,
		Source:     resolved.Source,
		Maintainer: resolved.Maintainer,
		Commit:     resolved.Commit,
		AuditMax:   report.Max().String(),
	})
	store.TrustMaintainer(resolved.Source, resolved.Maintainer, "via "+target)
	if err := store.Save(); err != nil {
		return result, err
	}
	result.Approved = true
	return result, nil
}
