package federate

import (
	"context"
	"strings"

	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// gate applies the trust verdict to one resolved package via the canonical
// trust.EvaluateResolved (so delegatedVerified bypasses the store consistently
// with the verify hook). A needs-review result is dropped in "enforce"; in "warn"
// it is annotated only when it violates an EXISTING approval (e.g. a maintainer
// change), never for the normal "never reviewed" state, which would noisily prefix
// every upstream package.
func gate(st *trust.Store, mode, source string, delegatedVerified bool, p aurweb.Pkg) (aurweb.Pkg, bool) {
	v := st.EvaluateResolved(source, p.PackageBase, p.Maintainer, delegatedVerified)
	if v.Decision == trust.Trusted {
		return p, true
	}
	if mode == "enforce" {
		return p, false
	}
	// A non-trusted verdict means the store was consulted, so st is non-nil here.
	if _, approved := st.Approval(p.PackageBase); approved {
		p.Description = "[kayo: " + strings.Join(v.Reasons, "; ") + "] " + p.Description
	}
	return p, true
}

// TrustUpstream wraps the real-AUR upstream so its results pass through the same
// trust gate (source "aur"). It embeds *aurweb.AURUpstream, so Suggest, the git
// helpers, and the dump source are promoted unchanged; only Info/Search gate.
type TrustUpstream struct {
	*aurweb.AURUpstream
	Store *trust.Store
	Mode  string
}

func (u *TrustUpstream) Info(ctx context.Context, names []string) ([]aurweb.Pkg, error) {
	pkgs, err := u.AURUpstream.Info(ctx, names)
	if err != nil {
		return nil, err
	}
	return u.gateAll(pkgs), nil
}

func (u *TrustUpstream) Search(ctx context.Context, by aurweb.By, arg string) ([]aurweb.Pkg, error) {
	pkgs, err := u.AURUpstream.Search(ctx, by, arg)
	if err != nil {
		return nil, err
	}
	return u.gateAll(pkgs), nil
}

func (u *TrustUpstream) gateAll(pkgs []aurweb.Pkg) []aurweb.Pkg {
	out := pkgs[:0]
	for _, p := range pkgs {
		if gp, keep := gate(u.Store, u.Mode, "aur", false, p); keep {
			out = append(out, gp)
		}
	}
	return out
}
