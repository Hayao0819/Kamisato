# Ayaka | 綾華

Ayaka is a command line front end. It builds packages locally and deploys them
to Ayato or Blinky.

## Features

- Build all packages within chroot environment
- Upload built binary to blinkyd server
- Ayaka works as client of blinkyd
- Signing packages with GPG key

## 動作環境

AyakaはArchLinux環境下でのみ動作します。Manjaroでの動作はテストしていません。CachyOS版のPacmanでは動作しません。

## 使い方

### 設定ファイル

`ayaka`は2つの設定ファイルで動作が決定されます。

#### `.ayakarc.json`

CLI設定のためのファイル

```json
{
    "repodir": "../example/src/myrepo",
    "destdir": "../example/out"
}
```

#### `repo.json`

リポジトリ設定のためのファイル

```json
{
    "name": "myrepo",
    "maintainer": "hayao <shun819.mail at gmail.com>",
    "server": "example.com/myrepo"
}
```

サンプルは[../example/src/myrepo/repo.json](../example/src/myrepo/repo.json)にあります。

### サブコマンド

- `build` 全てのパッケージをchroot環境内でビルドします
- `list` パッケージの一覧を表示します

## Todo

- Mirroring repository to another server
