package service

import (
	"reflect"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func TestPackageMetadataIsDetachedFromParserModel(t *testing.T) {
	t.Parallel()
	info := &raiou.PKGINFO{
		PkgName:     "name",
		PkgBase:     "base",
		PkgVer:      "1.0-1",
		PkgDesc:     "description",
		URL:         "https://example.invalid",
		BuildDate:   123,
		Packager:    "Packager",
		Size:        456,
		Arch:        "x86_64",
		License:     []string{"license"},
		Replaces:    []string{"replaces"},
		Group:       []string{"group"},
		Conflict:    []string{"conflict"},
		Provides:    []string{"provides"},
		Backup:      []string{"backup"},
		Depend:      []string{"depend"},
		OptDepend:   []string{"optdepend"},
		MakeDepend:  []string{"makedepend"},
		CheckDepend: []string{"checkdepend"},
		XData:       map[string]string{"key": "value"},
		PkgType:     "pkg",
	}
	want := domain.PackageMetadata{
		PkgName:     info.PkgName,
		PkgBase:     info.PkgBase,
		PkgVer:      info.PkgVer,
		PkgDesc:     info.PkgDesc,
		URL:         info.URL,
		BuildDate:   info.BuildDate,
		Packager:    info.Packager,
		Size:        info.Size,
		Arch:        info.Arch,
		License:     []string{"license"},
		Replaces:    []string{"replaces"},
		Group:       []string{"group"},
		Conflict:    []string{"conflict"},
		Provides:    []string{"provides"},
		Backup:      []string{"backup"},
		Depend:      []string{"depend"},
		OptDepend:   []string{"optdepend"},
		MakeDepend:  []string{"makedepend"},
		CheckDepend: []string{"checkdepend"},
		XData:       map[string]string{"key": "value"},
		PkgType:     info.PkgType,
	}

	got := packageMetadata(info)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packageMetadata() = %#v, want %#v", got, want)
	}

	info.License[0] = "changed"
	info.Replaces[0] = "changed"
	info.Group[0] = "changed"
	info.Conflict[0] = "changed"
	info.Provides[0] = "changed"
	info.Backup[0] = "changed"
	info.Depend[0] = "changed"
	info.OptDepend[0] = "changed"
	info.MakeDepend[0] = "changed"
	info.CheckDepend[0] = "changed"
	info.XData["key"] = "changed"
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packageMetadata() aliases parser-owned data: got %#v, want %#v", got, want)
	}
}
