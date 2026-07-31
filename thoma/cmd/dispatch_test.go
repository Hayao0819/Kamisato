package cmd

import (
	"testing"

	"github.com/spf13/pflag"
)

// parseArgs runs argv through the real thoma command's flag set, exactly as run
// does, so the tests exercise the command's declared flags rather than a private
// parser.
func parseArgs(t *testing.T, args []string) *pflag.FlagSet {
	t.Helper()
	f := RootCmd().Flags()
	if err := f.Parse(args); err != nil {
		t.Fatal(err)
	}
	return f
}

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
		{"repackage", []string{"--repackage"}, false},
		{"repackage short", []string{"-R"}, false},
		{"noarchive", []string{"--noarchive"}, false},

		// paru-style bundled short flags.
		{"paru extract -ofA", []string{"-ofA", "-C"}, false},
		{"paru build -feA", []string{"-feA", "--noconfirm", "--noprepare", "--holdver"}, true},

		// makepkgconf forwarded with a build is still a build.
		{"build with --config", []string{"--config", "/etc/makepkg.conf", "-f", "--noconfirm", "--noextract", "--noprepare", "--holdver"}, true},

		{"no args", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRemoteBuild(parseArgs(t, tc.args)); got != tc.want {
				t.Errorf("isRemoteBuild(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestBuildscriptArg(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-p", "PKGBUILD.custom"}, want: "PKGBUILD.custom"},
		{args: []string{"--buildscript=PKGBUILD.alt"}, want: "PKGBUILD.alt"},
		{args: []string{"-f"}, want: "PKGBUILD"},
	} {
		if got, _ := parseArgs(t, test.args).GetString("buildscript"); got != test.want {
			t.Errorf("buildscript from %v = %q, want %q", test.args, got, test.want)
		}
	}
}

func TestRejectRoot(t *testing.T) {
	if err := rejectRoot(0); err == nil {
		t.Fatal("root was accepted")
	}
	if err := rejectRoot(1000); err != nil {
		t.Fatalf("non-root was rejected: %v", err)
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
	}
	for _, tc := range cases {
		if got, _ := parseArgs(t, tc.args).GetString("config"); got != tc.want {
			t.Errorf("--config from %v = %q, want %q", tc.args, got, tc.want)
		}
	}
}

func TestParseRejectsMissingFlagValue(t *testing.T) {
	for _, arg := range []string{"--config", "--buildscript", "--dir", "--key"} {
		flags := RootCmd().Flags()
		if err := flags.Parse([]string{arg}); err == nil {
			t.Errorf("%s was accepted without a value", arg)
		}
	}
}

func TestParseRejectsUnknownFlag(t *testing.T) {
	flags := RootCmd().Flags()
	if err := flags.Parse([]string{"--definitely-invalid"}); err == nil {
		t.Fatal("unknown flag was accepted")
	}
}

func TestRemoteBuildOptions(t *testing.T) {
	flags := parseArgs(t, []string{"--dir", "/tmp/pkg", "--ignorearch", "--nocheck", "--noverify", "--skipinteg"})
	options, err := remoteBuildOptions(flags)
	if err != nil {
		t.Fatal(err)
	}
	if options.Dir != "/tmp/pkg" || !options.IgnoreArch || options.RunCheck == nil || *options.RunCheck || options.RunVerify == nil || *options.RunVerify {
		t.Fatalf("options = %+v", options)
	}
	if !options.SkipChecksums || !options.SkipPGPCheck {
		t.Fatalf("skip options = %+v", options)
	}
}

func TestRemoteBuildRejectsUnsupportedFlags(t *testing.T) {
	for _, arg := range []string{"--install", "--sign", "--nosign", "--key=test", "--nodeps", "--log"} {
		if _, err := remoteBuildOptions(parseArgs(t, []string{arg})); err == nil {
			t.Errorf("%s was accepted", arg)
		}
	}
}
