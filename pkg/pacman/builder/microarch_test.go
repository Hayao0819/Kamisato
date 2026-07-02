package builder

import (
	"os"
	"strings"
	"testing"
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
				// The tier must apply to both C and C++ flags.
				if !strings.Contains(got, "CFLAGS") || !strings.Contains(got, "CXXFLAGS") {
					t.Errorf("microarchOverride(%q) = %q, want CFLAGS and CXXFLAGS", tt.tier, got)
				}
			}
		})
	}
}

func TestValidMicroarch(t *testing.T) {
	for _, tier := range []string{"", "x86_64_v2", "x86_64_v3", "x86_64_v4"} {
		if !ValidMicroarch(tier) {
			t.Errorf("ValidMicroarch(%q) = false, want true", tier)
		}
	}
	for _, tier := range []string{"x86_64_v9", "v3", "aarch64", "garbage"} {
		if ValidMicroarch(tier) {
			t.Errorf("ValidMicroarch(%q) = true, want false", tier)
		}
	}
}

// The staged override the container backend bind-mounts must carry the tier's
// -march for a v3 build and carry no -march at all for a default build.
func TestStageOverrideConfMicroarch(t *testing.T) {
	v3Path, cleanup, err := stageOverrideConf("x86_64_v3")
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

	defPath, cleanupDef, err := stageOverrideConf("")
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
