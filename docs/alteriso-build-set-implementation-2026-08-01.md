# alteriso向けローカルbuild実装

## 状態

`ALTERISO_BUILD_REQUIREMENTS.md`を入力仕様として、Ayakaの`build`と`plan`へ複数pkgbaseの依存解決を追加する。実装日は2026年8月1日。alteriso側のモジュール変更は、このリポジトリの対象外である。

今回の実装は、AURとローカルPKGBUILDを一つの依存グラフとして解決し、隔離された環境でビルドしたあと、一時pacmanリポジトリとschema version 1のmanifestを生成する。Ayato、Miko、repo-addは起動もインストールも不要。

当初は`ayaka build-set --aur-list ... --pkgbuild-root ...`として実装したが、このCLIはalterisoのファイル名と親ディレクトリ構成をAyakaへ漏らしていた。現在の契約では、alterisoが入力ファイルのコメント、除外、レイヤーを処理し、Ayakaへpkgnameのargvと個別のローカルソースを渡す。Cobra層は入力方法を判定して正規形へ変換し、下位層は`Packages`と`LocalSources`だけを受け取る。

## CLI

alterisoから呼ぶコマンドは`ayaka build`である。

```console
ayaka build package-a package-b \
  --arch i486 \
  --local-source /work/input/pkgbuild.any/local-a \
  --local-source /work/input/pkgbuild.i486/local-b \
  --pacman-conf /work/i486.pacman.conf \
  --repo alteriso-local \
  --output /work/repo \
  --manifest /work/result.json
```

入力の解決だけを確認する場合は`ayaka plan`を使う。

```console
ayaka plan \
  --arch x86_64 \
  package-a package-b \
  --local-source ./pkgbuild.any/local-a \
  --pacman-conf ./pacman.conf \
  --json
```

既存の設定済みソースリポジトリは`--source-repo`で明示する。位置引数はどちらの経路でもpkgnameとして扱う。

```console
ayaka build --source-repo extra package-a package-b
```

### `build`の入力とフラグ

| フラグ | 条件 | 内容 |
| --- | --- | --- |
| `<pkgname>...` | ローカルソースがない場合は必須 | ビルドするpkgname。個別のargvとして渡す |
| `--arch` | 省略可 | 対象CARCH。既定値は`x86_64` |
| `--local-source` | 反復可 | 一つのpkgbaseを表すローカルディレクトリ |
| `--source-repo` | 省略可 | `.ayakarc`に登録された既存ソースリポジトリ |
| `--pacman-conf` | 直接指定では必須 | 依存解決とビルドで共用する設定 |
| `--repo` | 省略可 | 一時リポジトリ名。既定値は`ayaka-local` |
| `--output` | 直接指定では必須 | 完成したリポジトリの出力先 |
| `--manifest` | 直接指定では必須 | 成功manifestの出力先 |
| `--backend` | 省略可 | 現在は`container`のみ |
| `--image` | 省略可 | `$arch`を展開するコンテナイメージ |
| `--timeout` | 省略可 | pkgbaseごとの制限時間 |
| `--work-dir` | 省略可 | 一時作業領域の親ディレクトリ |
| `--keep-work` | 省略可 | 成否にかかわらず作業領域を残す |
| `--log-dir` | 省略可 | pkgbase別ログを置く外部ディレクトリ |

`--source-repo`と`--local-source`は同時に指定できない。`--source-repo`がなければ、位置引数のpkgnameと`--local-source`を依存グラフへ渡す。両方が空なら使用方法のエラーとし、alterisoはAyakaを呼ばない。

対象CARCHとして受け付けるのは`x86_64`、`i486`、`i686`、`pentium4`、`aarch64`、`armv7h`である。最初のalteriso統合で完了対象となるのは要件どおり`x86_64`、`i486`、`i686`に限る。

### Cobraとapplicationの境界

`ayaka/cmd/build`は、`--source-repo`の有無で実行経路を選ぶ。直接指定では、既存ソースリポジトリ用の署名、diff、publishフラグを拒否する。設定済みソースリポジトリのビルドでは、`--local-source`、`--pacman-conf`、`--output`などの直接指定用フラグを拒否する。どちらの経路でも位置引数はpkgnameであり、最初の位置引数を設定内容に応じてリポジトリ名へ読み替えない。

直接指定の共通フラグは`ayaka/cli.DirectBuildFlags`が所有する。`build`と`plan`は同じ検証と変換を使い、次のrequestを`internal/buildset.Application`へ渡す。

```go
type PlanRequest struct {
    Arch         string
    Packages     []string
    LocalSources []string
    PacmanConf   string
    WorkDir      string
    KeepWork     bool
}
```

applicationには、Cobraのcommand、flag set、`--source-repo`、alterisoのファイル形式、入力モードを表すenumを渡さない。設定済みソースリポジトリのビルドは既存のserviceへ残し、Cobra層でだけ別経路として選ぶ。これにより、CLIの指定方法を変更してもbuild graphのrequestは変わらない。

## 入力の扱い

### pkgname一覧

Ayakaは`packages_aur*`や通常の`packages*`を読まない。alterisoがファイルを統合し、コメント、`!pkgname`、重複を処理した結果を位置引数として渡す。Ayakaは受け取ったpkgnameの妥当性を検査し、同じpkgnameを一度だけ解決する。

位置引数はpkgbaseではなくpkgnameである。split packageの二つの出力を指定しても、pkgbaseは一度だけcloneして一度だけビルドする。

### ローカルPKGBUILD

`--local-source`一つを一つのpkgbaseソースとして扱う。Ayakaは親ディレクトリを走査しない。ディレクトリには`PKGBUILD`が必要で、ローカルソースファイルもディレクトリごと作業領域へコピーする。元のプロファイルとGit作業ツリーはビルドに渡さない。

alteriso側では、プロファイルとモジュールを統合するときに同名ディレクトリを検出する。Ayakaは、異なる`--local-source`が同じpkgbaseを宣言した場合にエラーを返す。順番による上書きは行わない。

`.SRCINFO`のpkgbaseと全pkgnameは、ログ名や作業用pathへ使う前にpacman package名として検証する。ローカルファイルにpath区切りや相対pathを含む名前が書かれていても、workspace外のファイル操作には到達しない。

ローカルソースが対象アーキテクチャ向けに生成するpkgnameは、すべてmanifestの`install`へ入る。AURは一覧で指定されたpkgnameだけが明示導入対象となり、依存として取得したAURパッケージは入らない。

### pacman.conf

`pacman-conf`で`Include`を展開し、リポジトリ順、Server、SigLevel、Usageを確定する。設定の`Architecture`に対象CARCHが含まれない場合は、依存解決を始める前に停止する。

確定したリポジトリ順を依存解決に使い、同じ内容からビルドコンテナ用のpacman.confを生成する。ビルドホストの`/etc/pacman.conf`は使わない。追加リポジトリを`.ayakarc`の`builder.repositories`へ書く構成は直接指定のbuildでは拒否し、`--pacman-conf`へ一本化した。

`file://`のServerを使う場合は、そのリポジトリディレクトリだけをコンテナ内の同じ絶対パスへread-onlyでmountする。resolverが読んだローカルDBをビルド時のpacmanからも参照できる。`/build`、`/etc`、pacman cacheと重なる指定は、ビルド用mountを上書きしないよう拒否する。

pacmanのリポジトリ優先順位は設定ファイルの記載順で決まり、先にあるリポジトリが同名パッケージに優先する。この実装も同じ順序で探索する。仕様の根拠は[pacman.conf(5)](https://man.archlinux.org/man/pacman.conf.5.en)を参照。

## 依存解決

依存グラフのノードはpkgbase、照合単位はpkgnameと`provides`である。`.SRCINFO`から次の値を対象CARCHに合わせて読む。

- `depends`
- `makedepends`
- `checkdepends`
- split package固有の`depends`
- アーキテクチャ固有の上記フィールド
- `provides`

`optdepends`はグラフへ入れない。

提供元は次の順で探す。

1. 今回指定されたローカルソース
2. 今回ビルドすることが確定したAURソース
3. `pacman.conf`のリポジトリ。記載順に探索
4. AURの同名pkgname
5. AURの`provides`検索

同じ段階に複数のpkgbase候補が残る場合は候補名を出して停止する。AUR RPCの返却順では選ばない。リポジトリ内で複数のパッケージが同じ仮想依存を提供する場合も同様である。

バージョン比較にはlibalpm互換の`vercmp`を使う。epochとpkgrelを含む`.SRCINFO`の完全なバージョンを保持する。`foo>=2`を`provides = foo`で満たすことはできず、`provides = foo=2`のように比較可能な値が必要になる。

AURの`provides`検索結果は検索レスポンスだけで採用しない。候補をinfo APIで引き直してversioned providesを確認し、clone後の`.SRCINFO`でも再度照合する。

グラフ完成後に安定したトポロジカルソートを行う。循環があれば`a -> b -> a`の形で経路を返す。解決不能エラーには依存元pkgbaseと元の制約を含める。

## `.SRCINFO`と隔離

既存の`.SRCINFO`があれば、そのファイルを解析する。存在しない場合は、ビルド用コンテナにソースをread-onlyでmountし、コンテナ内の非rootユーザーで`makepkg --printsrcinfo`を実行する。生成結果は作業用コピーへだけ書く。[makepkg(8)](https://man.archlinux.org/man/makepkg.8.en)では、`--printsrcinfo`は標準出力へSRCINFOを生成するオプションとして定義されている。

直接指定のbuildは現在、chroot backendを拒否する。devtoolsの`makechrootpkg`はビルド前にホスト側のshellでPKGBUILDを読み、pkgbaseとpkgnameを取得するためである。`PKGBUILDをホスト上でsourceしない`という要件を優先し、x86_64もcontainer backendを既定にした。

既定イメージは次のとおり。

```text
ghcr.io/hayao0819/archlinux:$arch
```

`$arch`は`x86_64`、`i486`、`i686`などの対象CARCHに置き換える。i486、i686、pentium4のOCI platformは`linux/386`だが、イメージtag、pacmanのArchitecture、makepkgのCARCHは分けて渡す。イメージをdigest付きで指定することもできる。可変tagを指定した場合も、最初のpullでDockerからRepoDigestを取得できれば残りの処理はそのdigestへ固定し、manifestへ記録する。一つのbuild実行中に異なるimageが混ざることを防ぐためである。

## ビルド実行

plannerが確定した順序を作り直さず、そのまま実行する。pkgbaseごとに別の出力ディレクトリとログを用意し、container backendへ一件ずつ渡す。

先に生成した推移依存のパッケージは、後続コンテナへread-onlyでmountする。`pacman -U`は対象ファイルを一つのtransactionとして扱うため、split packageや相互に依存する複数出力でもファイル名順に左右されない。その後、同じpacman.confを使う`makepkg --syncdeps`がリポジトリ上の残りの依存を導入する。

コンテナへ渡すホスト資源はソース、成果物用ディレクトリ、明示されたcache、pacman.conf、pacman.confに列挙されたローカルリポジトリ、makepkg override、先に生成したパッケージに限る。Docker socketをビルドコンテナへmountしない。キャンセルまたはtimeout後の削除には、キャンセル済みcontextとは別の短いcontextを使う。

source copy、metadata output、package output、makepkg overrideは、すべて`--work-dir`から作ったworkspace内に置く。Docker daemonが別コンテナや別ホストで動く構成では、`--work-dir`にclientとdaemonの双方から同じ絶対pathで見える場所を指定する。ローカルdaemonでは省略できる。

## 成果物の検証

backendが成功しても、そのままrepoへ登録しない。各パッケージの`.PKGINFO`とファイル名を`.SRCINFO`へ照合する。

- pkgnameが対象アーキテクチャ向け出力に含まれること
- package本体と隣接する署名がsymlinkではなく通常ファイルであること
- `.SRCINFO`の全出力が一度ずつ生成されたこと
- versionがepochとpkgrelを含めて一致すること
- archが対象CARCHまたは`any`であること
- pkgbaseが記録されている場合は一致すること
- `pkgname-pkgver-pkgrel-arch.pkg.tar.*`と`.PKGINFO`が一致すること
- `.BUILDINFO`が存在すること

`.BUILDINFO`はパッケージから取り出し、完成repoの`buildinfo/<pkgbase>/<pkgname>.BUILDINFO`へ保存する。pkgbase別ログは、`--log-dir`を指定しなければ完成repoの`logs/`へ入る。`--log-dir`を指定した場合は失敗後も診断できるよう、その外部ディレクトリへ直接書く。

## repoとmanifestの確定

全pkgbaseのビルドと検証が終わってから、`--output`と同じ親ディレクトリにstaging repoを作る。パッケージと署名をコピーし、`internal/pacman/repo.NativeTool.RepoAddBatch`を一度だけ呼ぶ。完成条件は次の4ファイルが通常ファイルとして存在すること。

```text
<repo>.db
<repo>.files
<repo>.db.tar.gz
<repo>.files.tar.gz
```

`repo-add`が複数パッケージを一度に扱い、`.db`と`.files`を作る挙動は[repo-add(8)](https://man.archlinux.org/man/repo-add.8.en)と同じである。外部の`repo-add`バイナリは呼ばない。

検証済みstagingを`--output`へrenameし、その後manifestをatomic replaceで確定する。manifestの書き込みに失敗した場合は、この実行で作ったrepoを削除し、repoだけが成功結果として残る状態を避ける。既存の`--output`と`--manifest`は上書きせずエラーにする。失敗後に前回のmanifestを今回の結果と誤認しないためである。

`<output>.lock`と`<manifest>.lock`にはprocess-sharedなadvisory lockを使う。待機せず競合を返す。同じrepoだけでなく、異なるrepoから同じmanifestを更新する実行も衝突として扱う。lockファイルはunlock後も同じinodeのまま残し、別プロセスが異なるlockファイルを掴む状態を作らない。

manifest、外部ログ、work directoryをrepoの内部に指定することはできない。repoのrename前に内部パスを作ってしまうと、atomic publishの前提が崩れるためである。

## manifest schema 1

トップレベルには次を記録する。

- `schema_version`
- `arch`
- `backend`
- `build_environment.image`
- 取得できた場合は`build_environment.digest`
- `repository.name`、`repository.path`、`repository.database`
- alterisoが追加する`install`
- build順の`builds`

各buildにはpkgbase、source、明示要求を含むか、ログ、解決済み依存、生成パッケージを記録する。ローカルsourceは元の絶対pathとコピー内容のSHA-256、AUR sourceはclone URL、commit revision、`.git`を除いた内容のSHA-256を持つ。

各生成パッケージにはpkgname、version、arch、完成repo内の絶対path、SHA-256、`.BUILDINFO`のpath、署名があれば署名path、`explicit`を記録する。依存としてだけ生成したパッケージは`explicit: false`となる。

## alteriso側の接続

alterisoは`pre__make_packages`で全入力を一度に渡す。Ayakaが成功したらmanifestをschema 1として検証し、次のrepo stanzaを一時pacman.confの標準リポジトリより前へ入れる。

```ini
[alteriso-local]
SigLevel = Optional TrustAll
Server = file:///absolute/path/from/manifest
```

この`SigLevel`は一時repoのsectionだけに置く。既存リポジトリの設定は変更しない。次にmanifestの`install`を`buildmode_pkg_list`へ追加し、通常の`_make_packages`を呼ぶ。ISOのairootfsへAUR helper、build user、PKGBUILD、Docker socketを入れる処理は不要になる。

## 現在の制約

- 直接指定のbuildはcontainer backendのみ。devtools backendを安全に使うには、ホスト側でPKGBUILDを読まない実行経路が別途必要
- 成果物cacheは未実装。pacman package cacheとccacheは既存のcontainer設定を利用できる
- `--output`は新規pathに限る。成功済みrepoの置換や世代管理はalteriso側のwork directoryで行う
- pacman.confの解析にはホスト上の`pacman-conf`が必要。ビルド自体のpacmanとmakepkgはコンテナ内で実行する
- AURソースの信頼判定は行わない。PKGBUILDとローカルソースは任意コードとして隔離する

## 実装位置

複数pkgbaseを扱う処理は`internal/buildset`にまとめた。小さなapplication/domain/ports packageへ分割せず、次のファイル単位で役割を分けている。

| ファイル | 担当 |
| --- | --- |
| `application.go` | workspaceとplanのライフサイクル |
| `sources.go` | 個別ローカルsourceのcopy、AUR clone、`.SRCINFO`、source digest |
| `repositories.go` | pacman repo DBの取得と候補照合 |
| `resolver.go` | 優先順位、versioned provides、グラフ |
| `build.go` | backend実行、成果物検証、publish |
| `manifest.go` | schema 1 |

一件のpkgbaseをビルドする責務は`internal/pacman/builder.Backend`に残した。`internal/buildset`はbackendを選ばず、順序、中間生成物、repo、manifestを所有する。ただし、CLI compositionでは前述の理由からcontainerだけを許可している。

## 確認状況

2026年8月1日時点で、次を確認した。

- `go test -p 1 ./...`
- `go vet -p 1 ./...`
- `golangci-lint run --concurrency 1 ./...`
- `ayaka build --help`と`ayaka plan --help`の公開フラグ
- `ayaka build-set`、`--aur-list`、`--pkgbuild-root`が公開CLIに残っていないこと

実コンテナを使うpkgbaseビルドとalterisoからのE2Eは未実施である。x86_64、i686、i486の完了判定には、archdockerイメージ、pacmanリポジトリ、alterisoの一時pacman.confを含む統合テストが別途必要になる。
