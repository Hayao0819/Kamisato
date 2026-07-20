package raiou_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// Captured from makepkg 7.1.0 --printsrcinfo. The three package functions use
// +=, =, and =() respectively.
const makepkg71SplitSRCINFO = `pkgbase = raiou-compat
	pkgdesc = global description
	pkgver = 1.2.3
	pkgrel = 4
	epoch = 2
	url = https://example.invalid/global
	arch = x86_64
	arch = aarch64
	license = MIT
	depends = glibc
	source = global-source
	cksums = SKIP
	b2sums = SKIP
	source_x86_64 = x86-source
	depends_x86_64 = global-x86
	cksums_x86_64 = SKIP
	b2sums_x86_64 = SKIP

pkgname = raiou-compat
	pkgdesc = appended description
	depends = glibc
	depends = appended
	depends_x86_64 = global-x86
	depends_x86_64 = appended-x86

pkgname = raiou-compat-replace
	depends = replacement
	depends_x86_64 = replacement-x86

pkgname = raiou-compat-empty
	pkgdesc =
	url =
	license =
	depends =
	depends_x86_64 =
`

func TestParseSrcinfoMakepkg71Fields(t *testing.T) {
	si, err := raiou.ParseSrcinfoString(makepkg71SplitSRCINFO)
	if err != nil {
		t.Fatalf("ParseSrcinfoString: %v", err)
	}

	if got := si.CKSums[""]; !reflect.DeepEqual(got, []string{"SKIP"}) {
		t.Errorf("CKSums[global] = %q, want [SKIP]", got)
	}
	if got := si.CKSums["x86_64"]; !reflect.DeepEqual(got, []string{"SKIP"}) {
		t.Errorf("CKSums[x86_64] = %q, want [SKIP]", got)
	}
	if got := si.B2Sums["x86_64"]; !reflect.DeepEqual(got, []string{"SKIP"}) {
		t.Errorf("B2Sums[x86_64] = %q, want [SKIP]", got)
	}

	rawEmpty := si.Packages[2]
	for field, values := range map[string][]string{
		"license": rawEmpty.License,
		"depends": rawEmpty.Depends[""],
	} {
		for _, value := range values {
			if strings.ContainsRune(value, '\x00') {
				t.Errorf("%s leaks go-srcinfo EmptyOverride: %q", field, value)
			}
		}
	}

	encoded, err := json.Marshal(si)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(encoded), `"pkgarch"`) || !strings.Contains(string(encoded), `"arch"`) {
		t.Errorf("SRCINFO JSON should use the format key arch: %s", encoded)
	}
}

func TestSplitPackagesResolvesMakepkgOverrides(t *testing.T) {
	si, err := raiou.ParseSrcinfoString(makepkg71SplitSRCINFO)
	if err != nil {
		t.Fatalf("ParseSrcinfoString: %v", err)
	}

	packages := si.SplitPackages()
	if len(packages) != 3 {
		t.Fatalf("SplitPackages len = %d, want 3", len(packages))
	}
	byName := make(map[string]raiou.SrcinfoPackage, len(packages))
	for _, pkg := range packages {
		byName[pkg.PkgName] = pkg
	}

	appended := byName["raiou-compat"]
	if appended.PkgDesc != "appended description" {
		t.Errorf("appended PkgDesc = %q", appended.PkgDesc)
	}
	if appended.URL != "https://example.invalid/global" {
		t.Errorf("appended URL = %q, want inherited URL", appended.URL)
	}
	if got := appended.Depends[""]; !reflect.DeepEqual(got, []string{"glibc", "appended"}) {
		t.Errorf("appended global Depends = %q", got)
	}
	if got := appended.Depends["x86_64"]; !reflect.DeepEqual(got, []string{"global-x86", "appended-x86"}) {
		t.Errorf("appended x86_64 Depends = %q", got)
	}

	replaced := byName["raiou-compat-replace"]
	if got := replaced.Depends[""]; !reflect.DeepEqual(got, []string{"replacement"}) {
		t.Errorf("replaced global Depends = %q", got)
	}
	if got := replaced.Depends["x86_64"]; !reflect.DeepEqual(got, []string{"replacement-x86"}) {
		t.Errorf("replaced x86_64 Depends = %q", got)
	}
	if !slices.Equal(replaced.License, []string{"MIT"}) {
		t.Errorf("replaced License = %q, want inherited MIT", replaced.License)
	}

	empty := byName["raiou-compat-empty"]
	if empty.PkgDesc != "" || empty.URL != "" {
		t.Errorf("explicitly empty scalars were not preserved: %+v", empty)
	}
	if len(empty.License) != 0 || len(empty.Depends) != 0 {
		t.Errorf("explicitly empty slices were not preserved: %+v", empty)
	}

	one, err := si.SplitPackage("raiou-compat-replace")
	if err != nil {
		t.Fatalf("SplitPackage: %v", err)
	}
	if !reflect.DeepEqual(*one, replaced) {
		t.Errorf("SplitPackage and SplitPackages disagree:\n one=%+v\nall=%+v", *one, replaced)
	}
	if _, err := si.SplitPackage("missing"); err == nil {
		t.Fatal("SplitPackage accepted an unknown package")
	}
}

func TestArchStringsForArch(t *testing.T) {
	values := raiou.ArchStrings{
		"":        {"common", ""},
		"x86_64":  {"x86"},
		"aarch64": {"arm"},
		"riscv64": nil,
	}
	if got, want := values.ForArch("x86_64"), []string{"common", "x86"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ForArch(x86_64) = %q, want %q", got, want)
	}
	if got, want := values.ForArch("aarch64"), []string{"common", "arm"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ForArch(aarch64) = %q, want %q", got, want)
	}
}

func TestParseSrcinfoToleratesFutureKnownArchField(t *testing.T) {
	data := "pkgbase = future\n" +
		"\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n" +
		"\tfuturefield_x86_64 = value\npkgname = future\n"
	if _, err := raiou.ParseSrcinfoString(data); err != nil {
		t.Fatalf("unknown field for a declared arch should remain forward-compatible: %v", err)
	}
}
