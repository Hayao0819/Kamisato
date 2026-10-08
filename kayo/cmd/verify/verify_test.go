package verifycmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/kayo/clonecache"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/service"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

func TestVerifyInputAndPolicyOutput(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "stdin", true: "arguments"}[explicit], func(t *testing.T) {
			policyErr := errors.New("blocked by policy")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cmd := newCommand(func(gotCtx context.Context, _ *kayoconfig.KayoConfig, names []string, strict bool) ([]service.Verification, error) {
				if gotCtx != ctx || !strict || !reflect.DeepEqual(names, []string{"one", "two"}) {
					t.Fatalf("context/input/strict not preserved: %v %v %v", gotCtx, names, strict)
				}
				return []service.Verification{{Name: "one", Source: "aur", Pkgbase: "one", Verdict: trust.Verdict{Decision: trust.NeedsReview, Reasons: []string{"unreviewed package"}}, Clone: clonecache.Result{Exists: true, Head: "123456789012extra", Pinned: "abcdefghijklmore"}}}, policyErr
			})
			cfgPath := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(cfgPath, []byte("addr = \"127.0.0.1:10713\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd.Flags().String("config", cfgPath, "")
			cmd.SetContext(ctx)
			cmd.SetIn(strings.NewReader("one\n\ntwo\n"))
			args := []string{"--strict"}
			if explicit {
				args = append(args, "one", "two")
			}
			cmd.SetArgs(args)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := cmd.Execute(); !errors.Is(err, policyErr) {
				t.Fatalf("error = %v", err)
			}
			want := "  REVIEW  one (aur) — unreviewed package\n  DRIFT   one (one) — clone cache at 123456789012, approved abcdefghijkl\n"
			if out.String() != want {
				t.Fatalf("output=%q, want %q", out.String(), want)
			}
		})
	}
}
