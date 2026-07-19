package domain

import "testing"

func TestAURTrustPolicyDecisions(t *testing.T) {
	t.Parallel()
	policy := NewAURTrustPolicy(AURTrustPolicySpec{
		TrustedMaintainers: []string{" Alice "},
		TrustedPkgbases:    []string{"pinned"},
	})

	tests := []struct {
		name       string
		pkgbase    string
		maintainer string
		want       AURTrustDecision
	}{
		{name: "pkgbase takes priority", pkgbase: "pinned", maintainer: "mallory", want: AURTrustByPkgbase},
		{name: "maintainer is canonicalized", pkgbase: "other", maintainer: "ALICE", want: AURTrustByMaintainer},
		{name: "orphan is blocked", pkgbase: "other", maintainer: "", want: AURTrustBlocked},
		{name: "unknown is blocked", pkgbase: "other", maintainer: "mallory", want: AURTrustBlocked},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := policy.Decide(test.pkgbase, test.maintainer); got != test.want {
				t.Fatalf("Decide(%q, %q) = %v, want %v", test.pkgbase, test.maintainer, got, test.want)
			}
		})
	}
}

func TestAURTrustPolicyCopiesInputAndPermissiveIsExplicit(t *testing.T) {
	t.Parallel()
	maintainers := []string{"alice"}
	pkgbases := []string{"foo"}
	policy := NewAURTrustPolicy(AURTrustPolicySpec{
		TrustedMaintainers: maintainers,
		TrustedPkgbases:    pkgbases,
		AllowUntrusted:     true,
	})
	maintainers[0] = "mallory"
	pkgbases[0] = "bar"

	if got := policy.Decide("foo", "nobody"); got != AURTrustByPkgbase {
		t.Fatalf("mutated pkgbase input changed policy: %v", got)
	}
	if got := policy.Decide("other", "alice"); got != AURTrustByMaintainer {
		t.Fatalf("mutated maintainer input changed policy: %v", got)
	}
	if got := policy.Decide("other", "mallory"); got != AURTrustUntrusted {
		t.Fatalf("permissive fallback = %v, want AURTrustUntrusted", got)
	}
}
