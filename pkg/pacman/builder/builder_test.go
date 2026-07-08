package builder

import "testing"

func TestIsPackageFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"zst package", "linux-6.1.0-1-x86_64.pkg.tar.zst", true},
		{"xz package", "linux-6.1.0-1-x86_64.pkg.tar.xz", true},
		{"gz package", "linux-6.1.0-1-x86_64.pkg.tar.gz", true},
		{"bz2 package", "linux-6.1.0-1-x86_64.pkg.tar.bz2", true},
		{"uncompressed package", "linux-6.1.0-1-x86_64.pkg.tar", true},
		{"sig file", "linux-6.1.0-1-x86_64.pkg.tar.zst.sig", false},
		{"plain tarball", "archive.tar.gz", false},
		{"random file", "README.md", false},
		{"empty string", "", false},
		{"extension only", ".pkg.tar.zst", false},
		{"PKGBUILD", "PKGBUILD", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPackageFile(tt.filename); got != tt.want {
				t.Errorf("isPackageFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestArchToCHOST(t *testing.T) {
	for arch, want := range map[string]string{
		"x86_64":   "x86_64-pc-linux-gnu",
		"i686":     "i686-pc-linux-gnu",
		"i486":     "i486-pc-linux-gnu",
		"pentium4": "i686-pc-linux-gnu", // archlinux32 pentium4 uses the i686 toolchain
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
			if p.OS != tt.wantOS {
				t.Errorf("archToPlatform(%q).OS = %q, want %q", tt.arch, p.OS, tt.wantOS)
			}
			if p.Architecture != tt.wantArch {
				t.Errorf("archToPlatform(%q).Architecture = %q, want %q", tt.arch, p.Architecture, tt.wantArch)
			}
			if p.Variant != tt.wantVar {
				t.Errorf("archToPlatform(%q).Variant = %q, want %q", tt.arch, p.Variant, tt.wantVar)
			}
		})
	}
}
