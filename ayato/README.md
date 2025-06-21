# Ayato | 綾人

Ayato is a Blinkyd compatible backend for ayaka and blinky.It handles package hosting
with automatic database updates.

## Features

- Upload package file with `blinky` or `ayaka` command
- Delete package file with `blinky` command
- Auto update repository database
- Store binaries in S3 or local
- Store metadata in SQL or local

- Minimal Pacman dependency

## Dependencies

`ayato` depends on the `repo-add` and `repo-remove` commands, which are included
in the `pacman` package.

Your entire system does not need to be managed by `pacman`; you can simply install
the `pacman` package on any distribution.

This is one of the advantages of `ayato` not being responsible for package compilation
itself.

- ArchLinux/Manjaro: You are probably already using Pacman
- Debian/Ubuntu: `apt install pacman-package-manager`
- AlpineLinux: `apk add pacman`
- Fedora: `dnf install pacman`

In other words, you can host the Pacman package manager on almost any distribution.
(I personally do not like using ArchLinux for server purposes.)

## Todo

- Implement basic features
  - Store metadata
  - Provide stored data
- Multi-arch support
- GPG sign support
- API for lumine
