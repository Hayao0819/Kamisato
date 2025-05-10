# Example

リポジトリのソースコードと、実際にBlinky(Ayato)で構築されるサーバーの動作例です。

## src/myrepo

リポジトリソースコードの例です。`PKGBUILD`が格納された各ディレクトリと同じ階層に`repo.json`があります。

## out

Ayakaによってビルドされたパッケージバイナリはこのディレクトリに格納されます。

バイナリの保管場所はAyakaの`destdir`で設定されます。

## srv

Blinkyの`REPO_PATH`に相当するディレクトリです。
