package cmd

import "testing"

func TestIsRemoteBuild(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		// yay's four invocations for one AUR build.
		{"yay source download", []string{"--verifysource", "--skippgpcheck", "-f", "-Cc"}, false},
		{"yay extract+pkgver", []string{"--nobuild", "-f", "-C"}, false},
		{"yay packagelist", []string{"--packagelist", "--ignorearch"}, false},
		{"yay heavy compile", []string{"-f", "--noconfirm", "--noextract", "--noprepare", "--holdver", "-c"}, true},
		{"yay heavy compile ignorearch", []string{"-f", "--noconfirm", "--noextract", "--noprepare", "--holdver", "--ignorearch"}, true},
		{"yay skip-build branch", []string{"--nobuild", "--noextract", "--ignorearch"}, false},

		// query / metadata invocations.
		{"printsrcinfo", []string{"--printsrcinfo"}, false},
		{"version long", []string{"--version"}, false},
		{"version short", []string{"-V"}, false},
		{"help", []string{"--help"}, false},

		// paru-style bundled short flags.
		{"paru extract -ofA", []string{"-ofA", "-C"}, false},
		{"paru build -feA", []string{"-feA", "--noconfirm", "--noprepare", "--holdver"}, true},

		// makepkgconf forwarded with a build is still a build.
		{"build with --config", []string{"--config", "/etc/makepkg.conf", "-f", "--noconfirm", "--noextract", "--noprepare", "--holdver"}, true},

		{"no args", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRemoteBuild(tc.args); got != tc.want {
				t.Errorf("isRemoteBuild(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestPkgName(t *testing.T) {
	cases := map[string]string{
		"foo-1.0-1-x86_64.pkg.tar.zst":             "foo",
		"foo-bar-1.0-1-x86_64.pkg.tar.zst":         "foo-bar",
		"python-foo-1.2.3-4-any.pkg.tar.xz":        "python-foo",
		"foo-2:1.0-1-any.pkg.tar.zst":              "foo", // epoch in version
		"foo-git-r123.abcdef-1-x86_64.pkg.tar.zst": "foo-git",
	}
	for in, want := range cases {
		if got := pkgName(in); got != want {
			t.Errorf("pkgName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConfigArg(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--config", "/etc/makepkg.conf", "-f"}, "/etc/makepkg.conf"},
		{[]string{"--config=/x/makepkg.conf"}, "/x/makepkg.conf"},
		{[]string{"-f", "--noconfirm"}, ""},
		{[]string{"--config"}, ""}, // dangling, no value
	}
	for _, tc := range cases {
		if got := configArg(tc.args); got != tc.want {
			t.Errorf("configArg(%v) = %q, want %q", tc.args, got, tc.want)
		}
	}
}
