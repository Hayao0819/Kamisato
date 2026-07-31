# 共有パッケージ構造の整理

最終更新: 2026-08-01

## 作業状態

- ブランチ: `package-structure-refactor`
- 開始地点: `b23366f`
- 状態: 完了
- 目的: AyakaとKokomiの機能再編に先立ち、既存の共有パッケージから製品間の不要な依存を除く
- コミット: 未作成。ユーザーの許可があるまでコミットしない

この文書は、会話が圧縮された場合や別の担当者が作業を引き継ぐ場合の再開地点を兼ねる。各段階を終えたら、変更内容、検証結果、次に着手する項目を末尾の作業記録へ追記する。

## 背景

現在のリポジトリには133個のGoパッケージがあり、`internal`配下が16、`pkg`配下が24を占める。問題は数ではなく、`internal/conf`と`internal/client`が全製品の型と処理を一つのパッケージへ集めている点にある。

調査時点では、`internal/conf`に22個、`internal/client`に17個のパッケージが直接依存していた。Goはファイル単位ではなくパッケージ単位でコンパイルするため、LumineがLumine用設定だけを使っても、同じ`conf`パッケージにあるAyato、Miko、builder、clientの依存が入る。Ayaka、Ayato、Miko、Kayo、Lumine、Thomaの全バイナリが、推移的に次のパッケージを参照していた。

- `pkg/pacman/builder`
- `pkg/pacman/depend`
- `pkg/pacman/pkg`
- `pkg/pacman/repo`
- `pkg/pacman/reponame`
- `pkg/raiou`
- `pkg/safefile`

新しい`buildset`をこの依存関係へ追加すると、既存の混線を固定してしまう。先に共有パッケージを整理する。

## 採用する粒度

ディレクトリは分類ではなく、import境界が必要な場所にだけ作る。`application`、`domain`、`ports`、`interfaces`のような層別パッケージは作らない。

次のいずれかに該当するときだけパッケージを分ける。

1. 利用しない外部依存やCライブラリが別の製品へ伝播する。
2. 分離しないと依存循環が生じる。
3. backendや外部サービスの実装を差し替える。
4. 公開APIと非公開実装を分ける。
5. 保存形式や通信形式など、複数製品が同じ契約を共有する。

共有関数が数個あるだけなら、既存の利用側へ置く。interfaceは原則として利用側が定義する。構造整理後の`internal`と`pkg`の合計パッケージ数は、現在の40から増やさない。

リポジトリ内の複数製品から呼ばれるだけでは、`pkg`へ置く理由にならない。`pkg`は外部モジュールからのimportを想定し、互換性を維持するAPIだけに使う。外部利用の事例がまだなくても、Kamisato固有の設定や製品packageへ依存せず、単独で用途を説明でき、今後も公開契約として保守するものは置いてよい。条件を満たさない共有実装は`internal`へ置き、外部利用が決まった時点でAPIを設計して移す。

小さくても独立を認める例外は次のとおり。

- `internal/version`: linkerから値を設定し、全バイナリが参照する。
- `internal/kayoproto`: AyatoとKayoが永続化・署名対象の通信形式を共有する。
- `pkg/raiou`: 外部利用を案内しているALPM metadata codecである。

## 公開API

この作業では次だけを`pkg`に残す。

```text
pkg/
├── aurweb/
└── raiou/
```

`pkg/aurweb`はaurweb互換のHTTP handler、client、wire typeを一体で提供し、Ayato以外のhostでも利用できるため公開APIとして残す。rate limitの固定窓実装はKamisato内でしか使わない。`aurweb`の公開引数から汎用の`Policy`と`Decision`を除き、RPCの許可判定だけを受け取る`aurweb.RateLimitFunc`へ縮める。

次はKamisato内部の実装であり、`internal`へ移す。

- `pkg/nvcheck`
- `pkg/safefile`
- `pkg/pacman`以下

`pkg/httpx`は`internal`へ移さず削除する。HTTP methodや通信先を区別せず、同じtransportでGETとPOSTを自動再試行していたため、操作を所有するpackageの判断を上書きしていた。通常のHTTP clientは組み立て箇所で生成し、再試行は`internal/apiclient`など操作の再実行可否を判断できるpackageだけが行う。

公開パッケージが同一モジュールの`internal`実装をimportすることは許可する。ただし、公開関数、公開method、公開struct fieldへ`internal`の型を露出しない。公開パッケージからAyaka、Ayato、Mikoなどの製品パッケージへも依存しない。

## 目標構造

```text
pkg/
├── aurweb/
└── raiou/

internal/
├── buildset/                 # 後続作業。この構造整理では作らない
├── ayatoapi/                 # Ayato client、OAuth、endpoint保存
├── mikoapi/                  # Miko client、build DTO、signer通信
├── apiclient/                # 両clientが共有するHTTP transport
├── kayoproto/
├── config/                   # 設定ファイルを読む共通処理だけ
├── pacman/
│   ├── builder/
│   │   ├── bwrap/
│   │   ├── devtools/
│   │   └── docker/
│   ├── repo/                 # binary repository DBだけ
│   ├── source/               # PKGBUILD directoryと.SRCINFO操作
│   ├── host/                 # pacman.conf、local DB、download、makepkg.conf
│   ├── sign/
│   ├── keyring/
│   └── hook/
├── auth/
├── nvcheck/
├── safefile/
├── cliutil/
├── ginutil/
├── gitcmd/
├── limits/
└── version/

ayato/config/                 # Ayatoの設定型と既定値
ayato/blob/                   # Ayatoのblob契約とlocalfs/S3実装
ayato/httpapi/                # Ayato HTTP APIのerror responseとorigin処理
miko/config/                  # Mikoの設定型とlegacy migration
kayo/config/                  # Kayoの設定型とpath解決
```

`internal/pacman`本体はpacmanの値と純粋な処理を所有する。

- package artifact名と`.pkg.tar.*`の解析
- `SourcePackage`と`BinaryPackage`のmetadata view
- dependency constraint
- dependency graphとtopological sort
- repository名の検証
- soname解析

外部環境へ接続するコードだけをサブパッケージへ分ける。`host`は`pacman-conf`、libalpm、`pacman`コマンドに依存する。`builder`はDocker、devtools、bubblewrapへ依存する。`repo`はbinary repository DBを扱う。`source`はPKGBUILD directoryと`.SRCINFO`を扱う。

## 現行パッケージからの移動

| 現在 | 移動先 | 備考 |
|---|---|---|
| `pkg/pacman/pkg` | `internal/pacman` | package名を`pacman`へ変更 |
| `pkg/pacman/depend` | `internal/pacman` | AUR探索方針は後で`buildset`へ移す |
| `pkg/pacman/reponame` | `internal/pacman` | `ValidateRepositoryName`へ改名 |
| `pkg/pacman`直下 | `internal/pacman/host` | host pacmanへの接続 |
| `pkg/pacman/makepkgconf` | `internal/pacman/host` | 一つのhost packageへ統合 |
| `pkg/pacman/repo`のDB処理 | `internal/pacman/repo` | `.db`、`.files`、remote DB、merge |
| `pkg/pacman/repo`のsource処理 | `internal/pacman/source` | source探索、選択、`.SRCINFO`生成 |
| `pkg/pacman/builder` | `internal/pacman/builder` | 既存のbackend分離は維持 |
| `pkg/pacman/sign` | `internal/pacman/sign` | 挙動は変えない |
| `pkg/pacman/keyring` | `internal/pacman/keyring` | `sign`との統合は利用状況を見て判断 |
| `pkg/pacman/hook` | `internal/pacman/hook` | CLI構築は`internal/hookcmd`に残す |
| `pkg/httpx` | 削除 | timeoutは各client、retryは操作の所有packageが担当 |
| `pkg/ratelimit` | `pkg/aurweb`、`ayato/middleware` | in-memory実装とKV-backed実装を各利用側が所有。`aurweb`は許可判定関数だけ公開 |
| `pkg/nvcheck` | `internal/nvcheck` | KokomiとMikoが共有する内部処理 |
| `pkg/safefile` | `internal/safefile` | atomic writeとlock |
| `internal/client` | `internal/ayatoapi`、`internal/mikoapi` | service boundaryで二分する |
| `internal/protocol` | `internal/mikoapi` | MikoのHTTP DTOであるため統合 |
| `internal/serverstore` | `internal/ayatoapi` | endpointとcredentialの保存 |
| `internal/blinkyutils` | `internal/ayatoapi` | 旧Blinky形式のmigrationとして統合 |
| `internal/auth/oauth` | `internal/ayatoapi` | Ayato user clientの認証 |
| `internal/conf`のloader | `internal/config` | koanf provider、env変換、ファイル探索、source transformだけ |
| `internal/conf`のAyato設定 | `ayato/config` | 500行を超える設定群のため独立境界とする |
| `internal/conf`のMiko設定 | `miko/config` | builder migrationを含むため独立境界とする |
| `internal/conf`のKayo設定 | `kayo/config` | federation、overlay、path解決をまとめる |
| `internal/conf`のAyaka設定 | `ayaka/app`、`internal/pacman/source` | process設定はapp、`repo.json`はsource repositoryが所有 |
| `internal/conf`のLumine、Thoma設定 | `lumine/config`、`thoma/config` | commandファイルとの一対一対応を保つため設定を分離 |
| `ayato/repository/blob` | `ayato/blob` | repository実装の子ではないAyato内の保存境界として独立 |

`internal/errors`はこの移動と同時には全面廃止しない。利用箇所が多いため、触れたファイルから`errors`、`fmt.Errorf("...: %w")`へ置き換える。単独の大規模置換は別作業とする。

`cliutil`、`ginutil`、`gitcmd`は名前だけを理由に変更しない。依存の混線が解消されない場合に限って分割する。

## configの扱い

`internal/config`には、koanf providerの組み立て、環境変数名の変換、設定ファイル探索、読み込み前のsource transformだけを置く。

当初はLumineとThomaの設定を`cmd`へ置いていたが、commandファイルとの一対一対応を崩していた。現在は全製品の設定型を製品内の`config`または`app`へ置く。Ayakaのprocess設定は`ayaka/app`、`repo.json`は`internal/pacman/source`が所有する。

Ayatoのserviceとrepository、Mikoのserviceは製品設定型をimportしない。各packageは実行に必要な値だけを持つ`Settings`を定義し、各製品の`app`が設定ファイル型から変換する。広いgetter interfaceを挟まないため、設定項目が暗黙に増えず、typed nilを渡して起動後にpanicする余地もない。serviceとrepositoryのproduction codeは設定ファイル形式から切り離す。

## API clientの扱い

`internal/client`は次の二つへ分ける。

- `internal/ayatoapi`: Ayato user client、publisher client、OAuth、endpoint registry
- `internal/mikoapi`: Miko build client、build job DTO、remote signer client

base URL、認証header、credential付きredirectの拒否、token refreshは`internal/apiclient`へ一度だけ抽出した。retryもここで操作ごとに指定し、`RetryReplaySafe`を付けたreadだけを再試行する。注入する`http.Client`自体にはretryを持たせない。Ayato固有のlogin、publisher、repository read、endpoint保存は`ayatoapi`、Miko固有のbuild DTO、job操作、remote signerは`mikoapi`が所有する。Kayoだけが使うcatalog取得は`kayo/ayatosrc`が所有し、OAuth、keyring、Miko clientを含む`ayatoapi`へ依存させない。

## cmdのファイル構造

各バイナリ直下の`cmd/root.go`だけをroot commandの例外とする。それ以外の`cmd`配下では、production用のGoファイル一つにつきCobra commandを一つだけ定義し、ファイル名を`Use`の先頭語と揃える。子commandと同名のcommandが別階層にある場合は、ディレクトリでcommand pathを表す。

設定型、設定からserviceへ渡す値の変換、依存の組み立て、複数commandが使う表示処理は`cmd`へ置かない。設定型は各製品の`config`、compositionは`app`、Cobraに近い共通処理は`cli`が所有する。この規則により、`cmd`のファイル一覧と実際のcommand treeを対応させる。

この規則はリポジトリ直下の`cmd_structure_test.go`で検査する。root以外のファイルにCobra commandが複数ある場合、nested `root.go`が追加された場合、ファイル名と`Use`の先頭語が一致しない場合はtestを失敗させる。共通のhook install factoryを使う`install.go`だけはfactory呼び出しをcommand定義として扱う。

## 再調査結果

実装後のimport graphとpackage規模を再計測した。製品間の直接import、`internal`から製品packageへのimport、公開`pkg`から製品packageへのimportはない。現在の`pkg/aurweb`と`pkg/raiou`はproduction codeから`internal`もimportしておらず、公開境界に内部型が漏れる経路はない。

次の残存箇所を修正した。

- Kayoのsource組み立てとaudit対象解決を`kayo/cli`から`kayo/app`へ移した。`cli`にはCobraからの設定読込と表示だけを残した。
- Miko、Kayo、Lumineのserver起動を各`app`へ移し、`cmd/root.go`をcommand treeとflagの定義に限定した。
- `ayato/platform`を廃止した。ファイルstream契約は`ayato/blob`、HTTP error responseとorigin処理は`ayato/httpapi`、KV-backed rate limiterは`ayato/middleware`が所有する。
- `internal/ratelimit`を廃止した。`pkg/aurweb`のbounded in-memory limiterとAyatoのKV-backed limiterは実装もtestも共有しない。

行数の多いpackageは`ayato/handler`、`ayato/service`、`ayato/repository`、`internal/ayatoapi`、`miko/service`だった。前3者はendpoint、use case、永続化という境界が分かれ、各package内では同じservice stateまたはtransactionを共有している。`internal/ayatoapi`もAyato endpoint、credential、API clientという一つの外部service境界であり、Kayo専用のcatalog clientはすでに`kayo/ayatosrc`へ分離済みである。ファイル数だけを理由に分割すると小packageと受け渡し用interfaceが増えるため、今回は維持する。

`internal/errors`とlegacy config migrationは横断的な残存事項だが、この構造整理で一括変更しない。前者は触れた箇所から標準`errors`へ寄せ、後者は移行期限を決めて別変更で削除する。

## 実装順

### A. 記録と検証基準

- [x] 専用ブランチを作成
- [x] 本文書を作成
- [x] baselineの`go test ./...`を記録
- [x] package数と主要依存を再計測

### B. 低リスクの境界整理

- [x] `internal/version`からCobra commandを除く
- [x] `pkg/httpx`と`internal/httpx`を削除し、HTTP clientの所有権を利用側へ戻す
- [x] `pkg/ratelimit`を各利用側へ分け、`aurweb`の公開引数から内部型を除く
- [x] `pkg/nvcheck`を`internal/nvcheck`へ移す
- [x] `pkg/safefile`を`internal/safefile`へ移す

### C. pacman packageの再編

- [x] `pkg/pacman/pkg`、`depend`、`reponame`を`internal/pacman`へ統合
- [x] host依存を`internal/pacman/host`へ移す
- [x] builder、sign、keyring、hookを`internal/pacman`配下へ移す
- [x] binary repoとsource repoを分ける
- [x] `pkg/pacman`以下の参照をゼロにする

### D. API clientの再編

- [x] Miko DTOとclientを`internal/mikoapi`へ移す
- [x] Ayato client、OAuth、endpoint保存を`internal/ayatoapi`へ移す
- [x] 旧Blinky registry互換をAyato endpoint保存へ統合
- [x] `internal/client`、`internal/protocol`、`internal/serverstore`を削除

### E. configの再編

- [x] 共通loaderを`internal/config`へ抽出
- [x] 製品設定型を各製品の所有packageへ移す
- [x] Ayatoのserviceとrepository、Mikoのserviceから製品設定型への依存を除く
- [x] `internal/conf`を削除

### F. CLIとcompositionの再編

- [x] root以外のcommandファイルをsubcommand名と一対一にする
- [x] LumineとThomaの設定を各`config`へ移す
- [x] Ayato、Miko、Kayo、Lumineの起動組み立てを各`app`へ移す
- [x] Kayoのapplication処理を`cli`から`app`へ移す
- [x] commandファイル構造を検査するtestを追加する

### G. 検証

- [x] `gofmt`を実行
- [x] `go test ./...`
- [x] `go vet ./...`
- [x] 全単体バイナリをbuild
- [x] `pkg`から製品packageへの依存がないことを確認
- [x] `internal`と`pkg`のpackage総数が40以下であることを確認
- [x] 本文書の表と作業記録を実績へ更新

## 検証コマンド

```bash
go test ./...
go test . -run TestCommandFilesMatchCommands
go vet ./...
go build ./ayaka ./ayato ./miko ./kayo ./lumine ./thoma
go list ./internal/... ./pkg/... | wc -l
go list -f '{{.ImportPath}}|{{join .Imports ","}}' ./...
rg 'github.com/Hayao0819/Kamisato/pkg/pacman' --glob '*.go'
rg 'github.com/Hayao0819/Kamisato/internal/(client|conf|protocol|serverstore)(/|")' --glob '*.go'
rg 'github.com/Hayao0819/Kamisato/(internal|pkg)/(httpx|ratelimit)|github.com/Hayao0819/Kamisato/ayato/platform' --glob '*.go'
```

## 再開手順

1. `git status --short --branch`でブランチと既存の未追跡ファイルを確認する。
2. 本文書の「実装順」で最初に未完了の項目を探す。
3. 「作業記録」の最後に書かれた検証結果と次の対象を確認する。
4. 変更前に、対象packageの直接利用者を`rg`と`go list`で確認する。
5. 一段階ごとに対象packageのtestを実行し、成功後に本文書を更新する。

`.codex/`と`ALTERISO_BUILD_REQUIREMENTS.md`は作業開始前から存在する未追跡ファイルである。後者は構造整理後のpackage名とCLI境界に合わせて記述だけを更新する。

## 作業記録

### 2026-07-31: 開始

`master`の`b23366f`から`package-structure-refactor`を作成した。コード変更前に本書を作成した。次はbaseline testとpackage数の記録を行い、Bの低リスクな移動から着手する。

### 2026-07-31: baseline

コード変更前の`go test ./...`は成功した。package数は`internal`が16、`pkg`が24、合計40だった。次は`internal/version`からCobraを除き、`httpx`、`nvcheck`、`safefile`を`internal`へ移す。

### 2026-07-31: 低リスクの境界整理

`version.Command`を`cliutil.VersionCommand`へ移し、`internal/version`からCobra依存を除いた。`pkg/httpx`、`pkg/nvcheck`、`pkg/safefile`は、それぞれ同名の`internal` packageへ移した。関連する`internal`、`pkg/aurweb`、`pkg/pacman`、全製品packageのtestは成功した。次はpacman packageを再編する。

### 2026-07-31: pacman packageの再編

`pkg/pacman`以下を`internal/pacman`へ移した。旧`pkg`、`depend`、`reponame`は`internal/pacman`へ統合し、repository名検証は`ValidateRepositoryName`へ改名した。hostのpacmanとmakepkg.confへ接続する処理は`internal/pacman/host`へまとめた。

旧`repo`に同居していたsource探索、package選択、`.SRCINFO`生成は`internal/pacman/source`へ移した。remote DBとの差分とprune判定はbinary repositoryを所有する`internal/pacman/repo`に残した。`internal/pacman/...`とAyaka全体のtestは成功した。`internal`は35、`pkg`は3、合計38 packageとなった。次はAPI clientを再編する。

### 2026-07-31: API clientの再編

`internal/client`のHTTP実装を`internal/apiclient`へ抽出し、Ayato固有の操作を`internal/ayatoapi`、Miko固有の操作とwire DTOを`internal/mikoapi`へ移した。Ayato clientはMiko build clientを埋め込み、従来どおりAyato経由でbuild APIを呼べる。

endpointとcredentialの保存、token refresh、OAuth login、device login、旧Blinky `servers.json`互換は`ayatoapi`へ統合した。`internal/client`、`internal/protocol`、`internal/serverstore`、`internal/blinkyutils`、`internal/auth/oauth`は削除した。移動後の`go test ./...`は成功した。

### 2026-07-31: configの再編

koanf loader、env変換、設定ファイル探索、source transformを`internal/config`へ移した。製品設定は`ayato/config`、`miko/config`、`kayo/config`、`ayaka/app`、`internal/pacman/source`、LumineとThomaの各`cmd`へ移し、`internal/conf`を削除した。

この段階ではAyatoのserviceとrepository、Mikoのserviceが、各packageで定義した`Config` interfaceだけを受け取る形にした。後続レビューでgetter interfaceも値型の`Settings`へ置き換えている。再編直後はリポジトリ全体が132 package、`internal`が33、`pkg`が3となった。

### 2026-07-31: 最終検証

`gofmt`、`go test ./...`、`go vet ./...`、`go build ./ayaka ./ayato ./miko ./kayo ./lumine ./thoma`はすべて成功した。廃止したpackageと旧`pkg/pacman`へのimportは0件で、公開`pkg`から製品packageへのimportも0件だった。Ayatoのservice/repositoryとMikoのserviceから製品configへの直接importも0件である。

最終的なpackage数は`internal`が33、`pkg`が3で合計36となり、開始時の40を下回った。リポジトリ全体も開始時の133から132へ減った。この構造整理に必要な作業は完了している。

### 2026-07-31: HTTP clientとrate limitの境界修正

最初の整理では、既存の`pkg/httpx`を公開範囲から外すことだけを考え、`internal/httpx`として残していた。しかし、このpackageは通信先やHTTP methodを区別せず、接続エラー、429、5xxをtransport層で再試行する。`internal/apiclient`が`NoRetry`を指定した操作も再試行され、webhookとCAPTCHAのPOSTも再送対象になっていた。

`httpx`を削除し、HashiCorpの`go-retryablehttp`と`go-cleanhttp`も依存から除いた。AUR、OIDC、nvcheck、Ayatoのupstream取得、Mikoのversion確認にはtimeout付きの標準`http.Client`を渡す。Ayato APIの再試行は`internal/apiclient`の`RetryReplaySafe`を指定したreadだけに限定した。webhookとCAPTCHAはサーバーが5xxを返しても1回しかPOSTしないことをテストで固定した。

この時点では`pkg/ratelimit`を`internal/ratelimit`へ移し、in-memory実装とKV-backed実装で固定窓の計算を共有していた。後続の再調査で、二つの実装は保存方式だけでなくcounterの寿命と失敗時の扱いも異なると判断し、`internal/ratelimit`自体を削除した。現在は`pkg/aurweb`と`ayato/middleware`がそれぞれの実装を所有する。

追加修正後の`go test ./...`、`go vet ./...`、`go build ./ayaka ./ayato ./miko ./kayo ./lumine ./thoma`は成功した。`httpx`、`pkg/ratelimit`、`go-retryablehttp`、`go-cleanhttp`のコード上の参照は0件である。package数は`internal`が33、`pkg`が2で合計35、リポジトリ全体は131となった。公開packageは`pkg/aurweb`と`pkg/raiou`だけである。

### 2026-07-31: レビュー指摘の修正

Kayoのcatalog取得を`internal/ayatoapi`から`kayo/ayatosrc`へ移し、catalog同期だけのためにOAuth、keyring、Miko API、pacman関連の依存を引き込まない構造にした。Ayato設定に残っていた未使用のbuild設定も削除した。

Ayatoのserviceとrepository、Mikoのserviceが受け取っていたgetter中心の`Config` interfaceは、各packageが所有する値型の`Settings`へ置き換えた。設定ファイルからの変換は各`cmd`のcomposition rootが担当する。これによりtyped nilによるconstructor後のpanicをなくし、Mikoをゼロ値設定で組み立てた場合もbuilder backendがcontainerになるよう既定値を固定した。

2026-07-08のターゲット構造は旧設計であることを冒頭に明記した。現在の`pkg`と`internal`の規則は本書を正とする。alteriso要件とAyaka/Kokomi再設計文書に残っていた`pkg/pacman/repo`表記も、現在の内部実装とCLI境界に合わせて修正した。

修正後の`go test ./...`、`go vet ./...`、`go build ./ayaka ./ayato ./miko ./kayo ./lumine ./thoma`は成功した。Ayato service、Ayato repository、Miko service、Kayo catalog同期、`internal/apiclient`、`internal/ayatoapi`のrace testも成功した。`kayo/ayatosrc`の依存グラフには`internal/ayatoapi`、`internal/mikoapi`、`internal/pacman`、OAuth、keyringが含まれない。package数は`internal`が33、`pkg`が2、リポジトリ全体が131である。

### 2026-07-31: CLI構造と残存共有境界の整理

各バイナリ直下の`cmd/root.go`を除き、command一つにつきファイル一つとした。Ayakaの`key subkey`とKayoの`trust whitelist`はcommand pathをディレクトリで表し、Ayatoの`audit`、`gc`、`keygen`、Mikoの`apikey generate`は別ファイルへ分けた。nested `root.go`は0件である。リポジトリ直下に構造検査testを追加し、この対応を今後も維持する。

設定は`lumine/config`と`thoma/config`へ移した。Ayato、Miko、Kayo、Lumineのserver組み立ては各`app`が担当する。Kayoのaudit対象解決とsource組み立ても`kayo/app`へ移し、`kayo/cli`にはCobraに近い設定読込と表示を残した。

`internal/ratelimit`は削除した。aurwebのin-memory limiterは`pkg/aurweb`、AyatoのKV-backed limiterは`ayato/middleware`が所有する。再調査で見つかった`ayato/platform`も廃止し、ファイルstream契約を`ayato/blob`、HTTP API表現を`ayato/httpapi`へ移した。

`gofmt`、`git diff --check`、`go test ./...`、`go vet ./...`、6バイナリのbuildは成功した。Ayato service、repository、middleware、Miko service、Kayo applicationとcatalog同期、aurweb、Ayato API clientのrace testも成功した。廃止packageへの参照、製品間の直接import、`internal`から製品packageへのimport、productionの`pkg`から`internal`または製品packageへのimportはいずれも0件である。

最終的なpackage数は`internal`が32、`pkg`が2、リポジトリ全体が138となった。全体の増加はcommand pathと製品内`app`、`config`を明示したためであり、共有境界である`internal`と`pkg`の合計は開始時の40から34へ減っている。

### 2026-07-31: 敵対的レビュー後の修正

HTTPの再試行は共通transportへ戻さず、処理を所有するpackageへ追加した。`pkg/aurweb`のupstream RPCとdump取得、Ayatoのupstream repository同期、`internal/nvcheck`のsource取得だけが、接続エラー、429、5xxを最大4回まで再試行する。失敗したresponse bodyは閉じ、待機はcontextで中断できる。webhook、CAPTCHA、build submitなどのPOSTは対象外である。

Ayakaでは、初期化済みの`App`をCobraのcontext valueから取得する方式をやめた。root commandが遅延初期化する`app.Runtime`を作り、必要なsubcommandと補完関数へconstructor引数として渡す。command構造testはAST上の任意の`Use` fieldではなく`cobra.Command` literalだけを読み、実際のcommand treeに到達できるpathかどうかも検査する。

Ayatoのaudit、orphan GC、migrationと、Mikoのversion確認、signer server起動は`cmd`から各`app`へ移した。`cmd`はflag、設定読込、入出力だけを扱う。Ayatoのrepository port、batch DTO、repository固有エラー契約は`service`が所有し、repositoryはその契約を実装する。既存のrepository API名には型aliasを残したが、productionのserviceからrepository packageへのimportは除いた。

repositoryが認証状態やtoken生成を所有しないようにした。device認証状態は`ayato/domain`へ移し、build log用tokenはhandlerが暗号学的乱数から生成してrepositoryへ保存する。`auth`、Ayatoのhandlerとmiddleware、Mikoのhandlerは製品設定型を直接受け取らず、各packageが必要な値だけを定義する。設定ファイル型からの変換は各`app`に置いた。

Thomaはmakepkg互換の判定を補った。`-p/--buildscript`で選んだbuild scriptをMikoへ送り、成果物一覧を得る`makepkg --packagelist`にも同じ指定を渡す。`-R/--repackage`と`--noarchive`は遠隔buildにせず、実際のmakepkgへ委譲する。makepkgと同様にrootでのbuildを拒否する。旧`repo.json`のURL移行は`i486`、`i686`、`pentium4`の末尾もarchitectureとして除去し、差分取得時にarchitectureが二重にならないようにした。

個別修正後に、rootのcommand構造test、Ayaka全体、Ayatoのauth・handler・middleware・repository・router・service、Mikoのapp・cmd・handler、Thoma、`internal/pacman`、`internal/nvcheck`、`pkg/aurweb`のtestを実行し、すべて成功した。

### 2026-07-31: 敵対的レビュー修正後の全体検証

`gofmt`、`git diff --check`、`go test ./...`、`go vet ./...`、`go build ./ayaka ./ayato ./miko ./kayo ./lumine ./thoma`はすべて成功した。Ayatoのservice、repository、handler、middleware、Mikoのservice、`pkg/aurweb`、`internal/nvcheck`はrace detector付きのtestも成功した。

import graphを再検査し、製品間の直接import、`internal`から製品packageへのimport、productionの`pkg`から`internal`または製品packageへのimportはいずれも0件だった。Ayatoのauth、handler、middlewareとMikoのhandlerからAyato設定型へのimportもなく、Ayato serviceからrepository packageへのimportもない。package数は`internal`が32、`pkg`が2、リポジトリ全体が138で、前回の構造整理完了時から増えていない。

### 2026-08-01: レビュー文書を受けた修正

`package-structure-refactor-review-2026-08-01.md`の指摘と、その後の敵対的レビュー結果を突き合わせた。確認できた実害は修正し、packageやinterfaceを機械的に増やす提案は採用しなかった。

HTTPの再試行方針は変えていない。`httpx`は復元せず、再実行できるGETを所有するpackageだけが再試行する。AUR、Ayato upstream、nvcheckに加え、GitHub OIDCのdiscoveryとJWKS取得を最大4回まで再試行するようにした。webhookとreCAPTCHAのPOSTは再送しない。`Retry-After`の秒数は`time.Duration`へ変換する前に30秒へ丸め、巨大値によるoverflowも防いだ。gosecの抑制はhelperの呼び出し元ではなく実際のHTTP sinkへ置き直した。

Ayakaの`Runtime`は、通常commandで初めて必要になったときだけ設定とsource repositoryを読む。source repository名とpackage名のshell completionは、候補を返すために必要な場合だけ同じ`Runtime`を初期化する。`version`は設定を読まず、設定エラーから独立した。

Thomaはmakepkgが実buildへ渡す`--dir`、`--ignorearch`、check、verify、checksum、PGP関連の指定をMikoのbuild requestへ明示的に載せる。Mikoは受け取った値をtargetとAUR dependencyの両方へ継承し、各backendが固定済みのmakepkg引数へ変換する。`--nocheck`ではcheck dependencyも解決しない。任意の引数文字列は受け取らない。`--install`、signing、`--key`、`--nodeps`はremote buildで黙って無視せず、未対応として拒否する。`--dir`指定時はbuild script、相対makepkg config、`--packagelist`の作業ディレクトリを同じ場所へ揃えた。

Mikoは負の`max_log_bytes`と`max_log_readers`を設定エラーにする。設定loaderを通さずにserviceやhandlerを組み立てた場合も、安全な既定値へ正規化する。SSE logは書き込み前にdeadlineを設定し、writeまたはflushの失敗で終了し、成功後にdeadlineを解除する。アイドル中の正常なstreamが直前のdeadlineで切れ続ける状態を避けた。

各実行ファイルは`internal/cliutil.Execute`を使い、失敗をstderrへ一度だけ表示する。flagと引数の利用法エラーはexit 2、実行時エラーはexit 1に揃えた。引数を取らないserver commandとone-shot commandには`NoArgs`を明示した。Thomaはmakepkg互換shimとして独自のエラー表示を維持する。

Ayatoのblob契約とlocalfs/S3実装は`ayato/repository/blob`から`ayato/blob`へ移した。serviceとhandlerも利用する保存境界であり、repository実装の子に見せる必要がないためである。deviceとlog tokenのrepository側interfaceは削除し、constructorは実装を返す。handlerは従来どおり、自分が使うmethodだけのinterfaceを所有する。

Kayoのinstall時trust判定は`kayo/cmd/verify`から既存の`kayo/app`へ移した。commandにはflag、設定読込、pacman hookのstdin、出力先の接続だけを残した。一方、全commandへconsumer interfaceを追加する案は採用していない。現在のcommandは値型の設定と既存serviceを使っており、差し替え需要のない箇所までinterface化すると、この作業で避けると決めた小packageと受け渡し型が増える。具体的なビジネスロジックの混入は個別に`app`または`service`へ移す。

そのほか、Ayakaの一時build directoryを作成直後からcleanup対象にし、client-side signingへ呼び出し元のcontextを渡した。Miko wire typeを参照するWeb CIとtygo設定の旧`internal/protocol`参照を直し、生成済みTypeScriptへmakepkg optionを反映した。`.codex/`をignoreし、不要なexport、誤置換されたcomment、古いpackage commentも整理した。`repo.json`のlegacy migrationとThomaのbuildscript対応は、別の実装要求で導入した互換機能なので維持する。入力資料の`ALTERISO_BUILD_REQUIREMENTS.md`も移動していない。

#### 再レビュー後の追加修正

Ayatoのpublication準備中にarchitectureを補完すると、追加でspoolしたpackageがcleanup対象に入る前のsliceを`defer`が保持していた。終了時点のsliceを参照するclosureへ変え、成功時にも一時ディレクトリが残らないことをtestで固定した。

Thomaは未知のflagをCobraで拒否し、remote buildで扱えないlog、install、signing、key、dependency関連の指定を明示的なエラーにする。build scriptはmakepkgの`-p`で渡す。remote buildはCobraから受け取ったcontextとstdout、stderrをそのまま使うため、呼び出し元によるcancelと出力差し替えが途中で失われない。

Mikoのjob logは、省略表示を含めて`max_log_bytes`を超えない。上限到達後とclose後の書き込みは保存せず、`io.Writer`としては受け取ったbyte数を返す。Docker backendはcontainer logのcopy処理を全return pathでjoinし、停止しないstreamは期限後にcloseしてから終了を待つ。これによりbuildが終わるたびにlog goroutineが残る経路を塞いだ。

CLIでは、実行処理を持たないcommand groupをhelp表示用のcommandとして扱う。引数なしはhelpを表示して成功し、未知の子commandはusage errorとしてexit 2になる。required flagとCobraのflag組み合わせ違反もexit 2へ分類し、Ayatoの`audit`には余分な引数を拒否する`NoArgs`を付けた。Ayakaのsource repository補完は未初期化の`Runtime`からも候補を返す。

Ayato、Miko、Kayo、Lumineのserver起動は、設定読込、logging設定、signal contextの生成を`cmd`に置き、router、worker、HTTP serverの組み立てと終了待ちを`app`に置いた。Kayoのupdate処理とAyakaのprune処理も`cmd`から既存の`app`、`service/plan`へ移した。pruneのdry-runはAyato認証を要求せず、空のsource package集合からrepository全体を削除しない。Kayo verifyはpackageを増やさず、source解決、AUR参照、公式repository照合、clone cache確認を関数依存としてまとめた。security上重要なfail-closed分岐を、実networkやhostのpacman DBなしでtestできる。

検証結果は次のとおり。並列のGo linkは容量16 GiBの`/tmp` tmpfsを使い切ったため、全体検証は`-p 1`で行った。6バイナリのbuildだけは、空きのある作業用`TMPDIR`を作成して実行し、終了後に削除した。

- `gofmt -l`は出力なし
- `git diff --check`は成功
- `go test -count=1 -p 1 ./...`は成功
- `go vet -p 1 ./...`は成功
- `golangci-lint run --concurrency 1 ./...`は0件
- `go build -p 1 ./ayaka ./ayato ./miko ./kayo ./lumine ./thoma`は成功
- `internal/cliutil`、Ayato service、Miko job log、Docker builder、Miko service、Thoma、Kayo app、Ayaka planのtestは`-race -count=1 -p 1`でも成功
- `pnpm run gen:types`、`pnpm run check`、`pnpm run typecheck`は成功
- `pnpm run test`は4 files、31 testsが成功
- `nix flake check --no-build`は成功

実行ファイルでも終了codeを確認した。`ayaka src`はhelpを表示して0、`ayaka src typo`と`ayato audit stray --prune`は2、Thomaの未知flagはmakepkg互換shim独自の扱いを維持して1となる。

import graphも再検査した。製品間の直接import、`internal`から製品packageへのimport、productionの`pkg`から`internal`または製品packageへのimportはいずれも0件である。package数は`internal`が32、`pkg`が2、全体が138で、今回の境界修正では増えていない。コミットは作成していない。
