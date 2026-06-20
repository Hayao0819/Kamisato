// パッケージメタ情報のアクセサ
package pkg

import (
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"strings"

	"github.com/samber/lo"
)

// --- SourcePackage ---

// Base は pkgbase を返します。
func (p *SourcePackage) Base() string {
	return p.info.PkgBase
}

// Version は epoch:pkgver-pkgrel 形式のフルバージョンを返します（vercmp 用）。
func (p *SourcePackage) Version() string {
	v := p.info.PkgVer
	if p.info.PkgRel != "" {
		v += "-" + p.info.PkgRel
	}
	if p.info.PkgEpoch != "" {
		v = p.info.PkgEpoch + ":" + v
	}
	return v
}

// Names は pkgbase と各サブパッケージ名を返します。
func (p *SourcePackage) Names() []string {
	names := []string{p.info.PkgBase}
	for _, pkg := range p.info.Packages {
		names = append(names, pkg.PkgName)
	}
	return lo.Uniq(names)
}

// PkgFileNames は makepkg --packagelist の出力からビルド成果物のファイル名を返します。
func (p *SourcePackage) PkgFileNames() ([]string, error) {
	stdout := new(bytes.Buffer)
	cmd := exec.Command("makepkg", "--packagelist", "OPTIONS=-debug")
	cmd.Dir = p.dir
	cmd.Stdout = stdout
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	l := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(l) == 0 {
		return nil, fmt.Errorf("no package found")
	}
	pkgs := make([]string, len(l))
	for i, pkg := range l {
		pkgs[i] = path.Base(strings.TrimSpace(pkg))
	}
	return pkgs, nil
}

// --- BinaryPackage ---

// Name は pkgname を返します。
func (p *BinaryPackage) Name() string {
	return p.info.PkgName
}

// Base は pkgbase を返します。
func (p *BinaryPackage) Base() string {
	return p.info.PkgBase
}

// Version は pkgver（.PKGINFO 記載のフルバージョン）を返します。
func (p *BinaryPackage) Version() string {
	return p.info.PkgVer
}

// Arch はパッケージのアーキテクチャを返します。
func (p *BinaryPackage) Arch() string {
	return p.info.Arch
}
