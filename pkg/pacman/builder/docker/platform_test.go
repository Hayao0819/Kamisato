package docker

import "testing"

func TestArchToCHOST(t *testing.T) {
	for arch, want := range map[string]string{
		"x86_64":   "x86_64-pc-linux-gnu",
		"i686":     "i686-pc-linux-gnu",
		"i486":     "i486-pc-linux-gnu",
		"pentium4": "i686-pc-linux-gnu",
	} {
		if got := archToCHOST(arch); got != want {
			t.Errorf("archToCHOST(%q) = %q, want %q", arch, got, want)
		}
	}
}

func TestArchToPlatform(t *testing.T) {
	tests := []struct {
		name     string
		arch     string
		wantOS   string
		wantArch string
		wantVar  string
		wantErr  bool
	}{
		{"x86_64", "x86_64", "linux", "amd64", "", false},
		{"aarch64", "aarch64", "linux", "arm64", "", false},
		{"armv7h", "armv7h", "linux", "arm", "v7", false},
		{"i486", "i486", "linux", "386", "", false},
		{"i686", "i686", "linux", "386", "", false},
		{"pentium4", "pentium4", "linux", "386", "", false},
		{"unsupported", "riscv64", "", "", "", true},
		{"empty", "", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := archToPlatform(tt.arch)
			if (err != nil) != tt.wantErr {
				t.Errorf("archToPlatform(%q) error = %v, wantErr %v", tt.arch, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if p.OS != tt.wantOS || p.Architecture != tt.wantArch || p.Variant != tt.wantVar {
				t.Errorf("archToPlatform(%q) = %s/%s/%s, want %s/%s/%s", tt.arch, p.OS, p.Architecture, p.Variant, tt.wantOS, tt.wantArch, tt.wantVar)
			}
		})
	}
}
