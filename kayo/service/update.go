package service

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/vcs/git"
	"github.com/Hayao0819/Kamisato/kayo/audit"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/gitserve"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

type UpdateResult struct {
	Resolved     Resolved
	Previous     trust.Approval
	Report       audit.Report
	ChangedFiles []string
	Approve      bool
	Approved     bool
}

func Update(ctx context.Context, cfg *kayoconfig.KayoConfig, target string, approve, force bool) (*UpdateResult, error) {
	resolved, cleanup, err := resolveForReview(ctx, cfg, target, "", approve)
	defer cleanup()
	if err != nil {
		return nil, err
	}

	store, err := trust.Open(cfg.ResolvedTrustStore())
	if err != nil {
		return nil, err
	}
	previous, ok := store.Approval(resolved.Pkgbase)
	if !ok {
		return nil, errors.NewErrf("%s is not tracked; use 'kayo trust add' first", resolved.Pkgbase)
	}
	report, err := audit.Scan(resolved.Dir)
	if err != nil {
		return nil, err
	}
	result := &UpdateResult{
		Resolved:     resolved,
		Previous:     previous,
		Report:       report,
		ChangedFiles: changedFiles(resolved.Dir, previous.Commit, resolved.Commit),
		Approve:      approve,
	}
	if !approve {
		return result, nil
	}
	if report.Max() >= audit.SevHigh && !force {
		return result, errors.NewErr("refusing to approve: high-severity findings (use --force)")
	}
	if err := resolved.RequirePinnedCommit(); err != nil {
		return result, err
	}
	if err := gitserve.Materialize(ctx, cfg.ServedRoot(), resolved.Pkgbase, resolved.Dir, resolved.Commit); err != nil {
		return result, errors.WrapErr(err, "failed to re-pin reviewed commit")
	}
	store.Approve(trust.Approval{
		Pkgbase:    resolved.Pkgbase,
		Source:     resolved.Source,
		Maintainer: resolved.Maintainer,
		Commit:     resolved.Commit,
		AuditMax:   report.Max().String(),
	})
	store.TrustMaintainer(resolved.Source, resolved.Maintainer, "via update "+target)
	if err := store.Save(); err != nil {
		return result, err
	}
	result.Approved = true
	return result, nil
}

func changedFiles(dir, from, to string) []string {
	if from == "" || to == "" {
		return nil
	}
	names, err := git.ChangedFiles(dir, from, to)
	if err != nil {
		return nil
	}
	return names
}
