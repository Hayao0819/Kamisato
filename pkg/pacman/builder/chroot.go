package builder

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	utils "github.com/Hayao0819/Kamisato/internal/utils"
)

// chrootBackend は devtools の clean-chroot フロー
// (<ArchBuild> -- makechrootpkg -c -- makepkg ...) でパッケージをビルドします。
// Arch ホスト上でのみ動作し、root/nspawn を必要とします。
type chrootBackend struct {
	opts Options
}

func newChrootBackend(opts Options) Backend {
	return &chrootBackend{opts: opts}
}

func (b *chrootBackend) Name() string {
	return "chroot"
}

func (b *chrootBackend) Build(ctx context.Context, spec Spec) (*Result, error) {
	if spec.ArchBuild == "" {
		return nil, errors.New("chroot backend requires a devtools wrapper (Spec.ArchBuild), e.g. extra-x86_64-build")
	}
	if spec.SrcDir == "" {
		return nil, errors.New("chroot backend requires Spec.SrcDir")
	}

	// Timeout が指定されていればビルド全体を打ち切る。
	if b.opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.opts.Timeout)
		defer cancel()
	}

	// ビルド前に既存パッケージを記録しておく。makechrootpkg は成果物を SrcDir
	// (= CWD) に残すため、前回ビルドの残骸や InstallPkgs として置かれた依存
	// パッケージを「今回の成果物」と誤認しないよう、差分だけを採用する。
	baseline, err := snapshotPackages(spec.SrcDir)
	if err != nil {
		return nil, err
	}

	target := &Target{
		Arch:        spec.Arch,
		ArchBuild:   spec.ArchBuild,
		InstallPkgs: spec.InstallPkgs,
	}
	// LogWriter が指定されていれば、ビルド出力をコンソールと同時に捕捉する。
	if spec.LogWriter != nil {
		target.Output = io.MultiWriter(os.Stdout, spec.LogWriter)
	}

	slog.Info("building package in clean chroot", "dir", spec.SrcDir, "archbuild", spec.ArchBuild, "arch", spec.Arch)
	if err := target.BuildContext(ctx, spec.SrcDir); err != nil {
		return nil, utils.WrapErr(err, "failed to build package in chroot")
	}

	// 今回新しく生成されたパッケージだけを採用する。
	built, err := collectNewPackages(spec.SrcDir, baseline)
	if err != nil {
		return nil, err
	}
	if len(built) == 0 {
		return nil, errors.New("no package files (*.pkg.tar.*) were produced")
	}

	return moveToOutDir(built, spec.SrcDir, spec.OutDir)
}

// moveToOutDir は built（絶対パス）を outDir へ移動し、最終的な絶対パスを返します。
// outDir が srcDir と同一（または空）なら移動せずそのまま返します。
func moveToOutDir(built []string, srcDir, outDir string) (*Result, error) {
	if outDir == "" {
		outDir = srcDir
	}
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to resolve src dir")
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to resolve out dir")
	}
	if absOut == absSrc {
		return &Result{Packages: built}, nil
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil {
		return nil, utils.WrapErr(err, "failed to create output directory")
	}
	packages := make([]string, 0, len(built))
	for _, p := range built {
		dst := filepath.Join(absOut, filepath.Base(p))
		if err := utils.MoveFile(p, dst); err != nil {
			return nil, utils.WrapErr(err, "failed to move package to output directory")
		}
		packages = append(packages, dst)
	}
	return &Result{Packages: packages}, nil
}

// snapshotPackages は dir 内に現在存在するパッケージファイル (*.pkg.tar.*) の
// ファイル名集合を返します。dir が存在しない場合は空集合を返します。
func snapshotPackages(dir string) (map[string]struct{}, error) {
	set := map[string]struct{}{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return set, nil
		}
		return nil, utils.WrapErr(err, "failed to snapshot package dir")
	}
	for _, entry := range entries {
		if entry.IsDir() || !isPackageFile(entry.Name()) {
			continue
		}
		set[entry.Name()] = struct{}{}
	}
	return set, nil
}

// collectNewPackages は dir 内のパッケージファイルのうち baseline に含まれない
// もの（= 今回のビルドで生成されたもの）の絶対パスを返します。署名ファイル
// (*.sig) は除外します。
func collectNewPackages(dir string, baseline map[string]struct{}) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to read package dir")
	}
	var pkgs []string
	for _, entry := range entries {
		if entry.IsDir() || !isPackageFile(entry.Name()) {
			continue
		}
		if _, ok := baseline[entry.Name()]; ok {
			continue
		}
		abs, err := filepath.Abs(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, utils.WrapErr(err, "failed to resolve package path")
		}
		pkgs = append(pkgs, abs)
	}
	return pkgs, nil
}

// pkgFileExts は Arch パッケージファイルの末尾拡張子です。
var pkgFileExts = []string{
	".pkg.tar.zst",
	".pkg.tar.xz",
	".pkg.tar.gz",
	".pkg.tar.bz2",
	".pkg.tar.lrz",
	".pkg.tar.lzo",
	".pkg.tar.Z",
	".pkg.tar",
}

// isPackageFile はファイル名がビルド成果物パッケージ (*.pkg.tar.*) かどうかを
// 返します。署名ファイル (*.sig) は成果物とみなしません。
func isPackageFile(name string) bool {
	if strings.HasSuffix(name, ".sig") {
		return false
	}
	for _, ext := range pkgFileExts {
		if len(name) > len(ext) && strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}
