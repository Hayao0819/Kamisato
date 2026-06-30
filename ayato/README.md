# Ayato | 綾人

Ayato is a Blinkyd compatible backend for ayaka and blinky. It hosts packages and
updates the repository database automatically, and proxies build requests to miko.

## Features

- Upload package file with `blinky` or `ayaka` command
- Delete package file with `blinky` command
- Auto update repository database
- Store binaries in S3 or local
- Store metadata in SQL or local

- Minimal Pacman dependency

## Dependencies

`ayato` writes the repository database in Go and never compiles packages itself,
so it runs on any system with no `pacman` installed. (I personally do not like
using ArchLinux for server purposes.)
