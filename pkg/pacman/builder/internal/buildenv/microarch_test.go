package buildenv

import (
	"os"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

func TestMicroarchOverride(t *testing.T) {
	tests := []struct {
		name     string
		tier     string
		want     string // substring the override must contain ("" = empty output)
		wantErr  bool
		wantNone bool // output must be empty
	}{
		{name: "default is empty", tier: "", wantNone: true},
		{name: "v2", tier: "x86_64_v2", want: "-march=x86-64-v2"},
		{name: "v3", tier: "x86_64_v3", want: "-march=x86-64-v3"},
		{name: "v4", tier: "x86_64_v4", want: "-march=x86-64-v4"},
		{name: "unknown tier errors", tier: "x86_64_v9", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := microarchOverride(tt.tier)
			if (err != nil) != tt.wantErr {
				t.Fatalf("microarchOverride(%q) err = %v, wantErr %v", tt.tier, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.wantNone && got != "" {
				t.Fatalf("microarchOverride(%q) = %q, want empty", tt.tier, got)
			}
			if tt.want != "" {
				if !strings.Contains(got, tt.want) {
					t.Errorf("microarchOverride(%q) = %q, want it to contain %q", tt.tier, got, tt.want)
				}
				if !strings.Contains(got, "CFLAGS") || !strings.Contains(got, "CXXFLAGS") {
					t.Errorf("microarchOverride(%q) = %q, want CFLAGS and CXXFLAGS", tt.tier, got)
				}
			}
		})
	}
}

func TestValidMicroarch(t *testing.T) {
	for _, tier := range []string{"", "x86_64_v2", "x86_64_v3", "x86_64_v4"} {
		if !builder.ValidMicroarch(tier) {
			t.Errorf("ValidMicroarch(%q) = false, want true", tier)
		}
	}
	for _, tier := range []string{"x86_64_v9", "v3", "aarch64", "garbage"} {
		if builder.ValidMicroarch(tier) {
			t.Errorf("ValidMicroarch(%q) = true, want false", tier)
		}
	}
}

func TestMakepkgOverrideLines(t *testing.T) {
	if got, err := MakepkgOverrideLines(builder.MakepkgConfig{}); err != nil || got != "" {
		t.Fatalf("MakepkgOverrideLines(zero) = %q, %v; want empty, nil", got, err)
	}

	got, err := MakepkgOverrideLines(builder.MakepkgConfig{
		Packager:     "Foo Bar <foo@example.com>",
		Microarch:    "x86_64_v3",
		CFlagsAppend: "-O3",
		Options:      []string{"!strip", "ccache", "x'); touch /tmp/injected; ('"},
		CompressZst:  "zstd -c -T0 --ultra -20 -",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"PACKAGER='Foo Bar <foo@example.com>'",
		"-march=x86-64-v3",
		"CFLAGS+=' -O3'",
		"CXXFLAGS+=' -O3'",
		"COMPRESSZST=('zstd' '-c' '-T0' '--ultra' '-20' '-')",
		"OPTIONS+=('!strip' 'ccache' 'x'\\''); touch /tmp/injected; ('\\''')",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("MakepkgOverrideLines missing %q in:\n%s", want, got)
		}
	}

	if _, err := MakepkgOverrideLines(builder.MakepkgConfig{Microarch: "x86_64_v9"}); err == nil {
		t.Error("MakepkgOverrideLines(unknown tier): want error, got nil")
	}
}

func TestStageOverrideConfMicroarch(t *testing.T) {
	v3Path, cleanup, err := StageOverrideConf(builder.MakepkgConfig{Microarch: "x86_64_v3"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, err := os.ReadFile(v3Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "-march=x86-64-v3") {
		t.Errorf("v3 override missing -march=x86-64-v3:\n%s", data)
	}

	defPath, cleanupDef, err := StageOverrideConf(builder.MakepkgConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupDef()
	defData, err := os.ReadFile(defPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(defData), "-march=") {
		t.Errorf("default override must inject no -march, got:\n%s", defData)
	}
}
