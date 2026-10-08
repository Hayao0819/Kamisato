package source

import "testing"

func TestParseGitSource(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		remote string
		ref    string
		ok     bool
	}{
		{
			name:   "git over https",
			value:  "zig::git+https://github.com/ziglang/zig.git",
			remote: "https://github.com/ziglang/zig.git",
			ref:    "HEAD",
			ok:     true,
		},
		{
			name:   "branch and signed query",
			value:  "git+ssh://git@example.com/project.git?signed#branch=next?signed",
			remote: "ssh://git@example.com/project.git",
			ref:    "next",
			ok:     true,
		},
		{
			name:   "git protocol",
			value:  "git://example.com/project.git#branch=main",
			remote: "git://example.com/project.git",
			ref:    "main",
			ok:     true,
		},
		{name: "pinned commit", value: "git+https://example.com/project.git#commit=abc", ok: false},
		{name: "pinned tag", value: "git+https://example.com/project.git#tag=v1", ok: false},
		{name: "non-git VCS", value: "hg+https://example.com/project", ok: false},
		{name: "ordinary archive", value: "https://example.com/source.tar.zst", ok: false},
		{name: "unknown fragment", value: "git+https://example.com/project.git#revision=1", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseGitSource(tt.value)
			if ok != tt.ok {
				t.Fatalf("ParseGitSource(%q) ok = %t, want %t", tt.value, ok, tt.ok)
			}
			if got.Remote != tt.remote || got.Ref != tt.ref {
				t.Errorf("ParseGitSource(%q) = %+v, want remote=%q ref=%q", tt.value, got, tt.remote, tt.ref)
			}
		})
	}
}
