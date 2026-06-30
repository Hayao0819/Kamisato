package alpm

import (
	"math/rand"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestVerCmp(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		// equal
		{"1.0", "1.0", 0},
		{"1", "1", 0},
		{"1.5.0", "1.5.0", 0},
		{"1.0-1", "1.0-1", 0},
		{"1:1.0", "1:1.0", 0},
		{"0:1.0", "0:1.0", 0},

		// numeric segments
		{"1.0", "1.1", -1},
		{"1", "2", -1},
		{"1.5.1", "1.5.2", -1},
		{"1.5.0", "1.5.1", -1},
		{"2.0", "2.0.1", -1},
		{"1.0.1", "1", 1},
		{"1.0.0", "1", 1},

		// segment lengths (more segments wins when prefixes match)
		{"1.5", "1.5.1", -1},
		{"1.5.1", "1.5.10", -1},
		{"1.5.0", "1.5", 1},

		// alpha segments
		{"1.0a", "1.0b", -1},
		{"1.5.a", "1.5.b", -1},
		{"1.5.b", "1.5.a", 1},

		// numeric outranks alpha within a segment, and a trailing alpha
		// segment (no separator) makes the version older
		{"1.5.1", "1.5.b", 1},
		{"1.0a", "1.0", -1},
		{"1.5.a", "1.5", 1},
		{"1.5.b", "1.5", 1},

		// mixed alphanumeric vs dotted (classic pacman case)
		{"1.0.1", "1.0.a", 1},
		{"1.0a", "1.0.a", -1},
		{"2.0a", "2.0.a", -1},

		// epoch decides first
		{"1:1.0", "2.0", 1},
		{"1:1.0", "1.0", 1},
		{"1.0", "1:1.0", -1},
		{"0:1.0", "1.0", 0},
		{"1:1.0", "1:1.1", -1},
		{"1:1.0", "2:1.1", -1},
		{"1:1.0", "0:1.0-1", 1},

		// pkgrel: only compared when both versions carry one
		{"1.0-1", "1.0-2", -1},
		{"1.0-2", "1.0-1", 1},
		{"1.0", "1.0-1", 0},
		{"1.0-1", "1.0", 0},
		{"1.5.0-1", "1.5.0-2", -1},
		{"1.5.0-1", "1.5.1-1", -1},
		{"1.5.0", "1.5.1-1", -1},
		{"1.5.b-1", "1.5.b", 0},
		{"1.5-1", "1.5.b", -1},

		// leading zeros are insignificant
		{"1.005", "1.5", 0},
		{"1.05", "1.5", 0},
		{"1.0006", "1.6", 0},
		{"1.010", "1.10", 0},

		// differing separators
		{"2.0", "2_0", 0},
		{"2.0_a", "2.0.a", 0},
		{"2___a", "2_a", 1},
	}

	for _, tt := range tests {
		got, err := VerCmp(tt.a, tt.b)
		if err != nil {
			t.Errorf("VerCmp(%q, %q) returned error: %v", tt.a, tt.b, err)
			continue
		}
		if got != tt.want {
			t.Errorf("VerCmp(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}

		rev, err := VerCmp(tt.b, tt.a)
		if err != nil {
			t.Errorf("VerCmp(%q, %q) returned error: %v", tt.b, tt.a, err)
			continue
		}
		if rev != -tt.want {
			t.Errorf("VerCmp(%q, %q) = %d, want %d (antisymmetry)", tt.b, tt.a, rev, -tt.want)
		}
	}
}

// TestVerCmpAgainstBinary cross-checks the pure-Go implementation against the
// reference vercmp binary over a deterministic pseudo-random sample. It is
// skipped when vercmp is not installed (non-Arch hosts).
func TestVerCmpAgainstBinary(t *testing.T) {
	bin, err := exec.LookPath("vercmp")
	if err != nil {
		t.Skip("vercmp binary not on PATH; skipping cross-check")
	}

	rng := rand.New(rand.NewSource(0x5ada))
	for i := 0; i < 1000; i++ {
		a := randomVersion(rng)
		b := randomVersion(rng)

		got, err := VerCmp(a, b)
		if err != nil {
			t.Fatalf("VerCmp(%q, %q) returned error: %v", a, b, err)
		}
		want := refVercmp(t, bin, a, b)
		if got != want {
			t.Errorf("VerCmp(%q, %q) = %d, vercmp = %d", a, b, got, want)
		}
	}
}

func refVercmp(t *testing.T, bin, a, b string) int {
	t.Helper()
	out, err := exec.Command(bin, a, b).Output()
	if err != nil {
		t.Fatalf("vercmp %q %q failed: %v", a, b, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("parsing vercmp output %q: %v", out, err)
	}
	return n
}

func randomVersion(rng *rand.Rand) string {
	const digits = "0123456789"
	const alpha = "abcdefghijklmnopqrstuvwxyz"
	const seps = ".._+-"

	var sb strings.Builder
	if rng.Intn(3) == 0 {
		sb.WriteString(strconv.Itoa(rng.Intn(4)))
		sb.WriteByte(':')
	}

	for s := 0; s < 1+rng.Intn(4); s++ {
		if s > 0 {
			sb.WriteByte(seps[rng.Intn(len(seps))])
		}
		set := digits
		if rng.Intn(2) == 0 {
			set = alpha
		}
		for k := 0; k < 1+rng.Intn(4); k++ {
			sb.WriteByte(set[rng.Intn(len(set))])
		}
	}

	if rng.Intn(3) == 0 {
		sb.WriteByte('-')
		sb.WriteString(strconv.Itoa(1 + rng.Intn(5)))
	}

	if sb.Len() == 0 {
		return "0"
	}
	return sb.String()
}
