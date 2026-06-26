package aurweb

import (
	"slices"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

const splitSrcinfo = `
pkgbase = mytool
	pkgver = 1.2.3
	pkgrel = 2
	epoch = 1
	arch = x86_64
	makedepends = go
	depends = glibc

pkgname = mytool
	depends = glibc>=2.34
	provides = mytool-bin

pkgname = mytool-extras
	pkgdesc = extra bits
	depends = mytool
`

func TestFromSrcinfoSplit(t *testing.T) {
	si, err := raiou.ParseSrcinfoString(splitSrcinfo)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	pkgs := FromSrcinfo(si, SrcinfoMeta{Maintainer: "me", LastModified: 100})
	if len(pkgs) != 2 {
		t.Fatalf("got %d packages, want 2", len(pkgs))
	}

	byName := map[string]Pkg{}
	for _, p := range pkgs {
		byName[p.Name] = p
	}

	mt, ok := byName["mytool"]
	if !ok {
		t.Fatal("missing mytool")
	}
	if mt.Version != "1:1.2.3-2" {
		t.Errorf("Version = %q, want 1:1.2.3-2", mt.Version)
	}
	if mt.PackageBase != "mytool" {
		t.Errorf("PackageBase = %q", mt.PackageBase)
	}
	if mt.Maintainer != "me" || mt.LastModified != 100 {
		t.Errorf("meta not applied: %+v", mt)
	}
	// global depends (glibc) merged with package depends (glibc>=2.34).
	if !slices.Contains(mt.Depends, "glibc") || !slices.Contains(mt.Depends, "glibc>=2.34") {
		t.Errorf("Depends = %v, want both glibc and glibc>=2.34", mt.Depends)
	}
	if !slices.Contains(mt.MakeDepends, "go") {
		t.Errorf("MakeDepends = %v, want go", mt.MakeDepends)
	}
	if mt.URLPath != "/cgit/aur.git/snapshot/mytool.tar.gz" {
		t.Errorf("URLPath = %q", mt.URLPath)
	}

	extras := byName["mytool-extras"]
	if extras.Description != "extra bits" {
		t.Errorf("extras Description = %q", extras.Description)
	}
	if !slices.Contains(extras.Depends, "mytool") {
		t.Errorf("extras Depends = %v", extras.Depends)
	}
}
