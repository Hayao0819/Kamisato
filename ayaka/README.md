# Ayaka | 綾華

Ayaka is a command line front end. It builds packages locally and deploys them
to Ayato or Blinky.

## Features

- Build all packages within chroot environment
- Upload built binary to blinkyd server
- Ayaka works as client of blinkyd
- Signing packages with GPG key

## Supported Environment

Ayaka works only on ArchLinux. Operation on Manjaro is not tested. It does not
work with Pacman from CachyOS.

## Usage

### Configuration files

`ayaka` is configured by two files.

#### `.ayakarc.json`

File for CLI configuration

```json
{
    "repodir": "../example/src/myrepo",
    "destdir": "../example/out"
}
```

#### `repo.json`

File for repository configuration

```json
{
    "name": "myrepo",
    "maintainer": "hayao <shun819.mail at gmail.com>",
    "server": "example.com/myrepo"
}
```

Sample is available at [../example/src/myrepo/repo.json](../example/src/myrepo/repo.json).

### Subcommands

- `build` Build all packages in chroot environment
- `list` Show package list

## Todo

- Mirroring repository to another server
