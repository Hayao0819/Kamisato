# Project Kamisato

[![Go Report Card](https://goreportcard.com/badge/github.com/Hayao0819/Kamisato)](https://goreportcard.com/report/github.com/Hayao0819/Kamisato)
![GitHub License](https://img.shields.io/github/license/Hayao0819/Kamisato)
![GitHub last commit](https://img.shields.io/github/last-commit/Hayao0819/Kamisato)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/Hayao0819/Kamisato)
[![Go Lint & Vet](https://github.com/Hayao0819/Kamisato/actions/workflows/golang-lint.yml/badge.svg)](https://github.com/Hayao0819/Kamisato/actions/workflows/golang-lint.yml)

Project Kamisato builds and distributes Arch Linux packages. It is a set of
components that run independently or together.

## Ayaka

Ayaka is the command-line client. It manages local PKGBUILD sources, builds them
locally or submits them to miko (through ayato) for a server-side build, and
inspects jobs and the published repository.

[REFER TO THE DOCUMENT](./ayaka/README.md)

## Ayato

Ayato is a Blinkyd compatible backend for ayaka and blinky. It hosts packages,
updates the repository database automatically, and proxies build requests to miko.

[REFER TO THE DOCUMENT](./ayato/README.md)

## Miko

Miko is the build server. It builds a PKGBUILD or git/AUR source in a throwaway
Arch container and uploads the result to ayato. Package signing is disabled by
default; deployments that need unattended publishing can explicitly enable a
local worker key or a separate signer service. Clients normally reach miko only
through ayato.

## Lumine

Lumine is a Next.js web frontend for ayato: browse and search the repository,
submit builds, and watch job logs and build-server status.

[REFER TO THE DOCUMENT](./lumine/web/README.md)

## Kayo

Kayo is a local aurweb-compatible overlay you point an AUR helper at. It
intercepts package resolution, federating trusted git overlays, other ayato
instances, and the upstream AUR, then gates installs through a supply-chain
trust store and a pacman hook that warns about, or in enforce mode blocks,
packages no one has reviewed.

[REFER TO THE DOCUMENT](./kayo/README.md)

## Thoma

Thoma is a drop-in makepkg shim. It offloads the compile to miko (through ayato)
and keeps the rest local, so an AUR helper keeps working on a low-powered
machine without building anything there.

[REFER TO THE DOCUMENT](./thoma/README.md)

## Raiou

Raiou is the metadata-parsing library the other components build on. It reads the
ALPM formats: .SRCINFO, .PKGINFO, .BUILDINFO, and repository desc entries.

[REFER TO THE DOCUMENT](./pkg/raiou/README.md)

## About Docker Images

The [Dockerfile](./Dockerfile) provides an Alpine Linux-based image with Project
Kamisato binaries pre-installed.

You can use this image as a base to create your own package repository image, or
launch servers using Docker Compose.

These image files are published on the following image registries:

- [`hayao0819/kamisato`](https://hub.docker.com/r/hayao0819/kamisato)
- [`ghcr.io/hayao0819/kamisato`](https://github.com/Hayao0819/Kamisato/pkgs/container/kamisato)

For example configurations, see the [example](./example/) directory.

## Special thanks

- <https://genshin.hoyoverse.com/ja/character/inazuma?char=0>
- [BrenekH/blinky](https://github.com/BrenekH/blinky)
