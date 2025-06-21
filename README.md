# Project Kamisato

[![Go Report Card](https://goreportcard.com/badge/github.com/Hayao0819/Kamisato)](https://goreportcard.com/report/github.com/Hayao0819/Kamisato)
![GitHub License](https://img.shields.io/github/license/Hayao0819/Kamisato)
![GitHub last commit](https://img.shields.io/github/last-commit/Hayao0819/Kamisato)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/Hayao0819/Kamisato)
[![Go Lint & Vet](https://github.com/Hayao0819/Kamisato/actions/workflows/golang-lint.yml/badge.svg)](https://github.com/Hayao0819/Kamisato/actions/workflows/golang-lint.yml)

Project Kamisato is a comprehensive solution for managing everything from package
development to deployment on Arch Linux.

It consists of multiple components, some of which can be used independently.

## Ayaka

Ayaka is a command line front end. It builds packages locally and deploys them
to Ayato or Blinky.

[REFER TO THE DOCUMENT](./ayaka/README.md)

## Ayato

Ayato is a Blinkyd compatible backend for ayaka and blinky.It handles package hosting
with automatic database updates.

[REFER TO THE DOCUMENT](./ayato/README.md)

## Lumine

Lumine is a web frontend for Ayato, built using Next.js, that displays server
status and searches for packages.

### Features

- Display package list
- Search packages
- Display Ayato backend server status
- Bug reporting for packages (mock function)

### About Docker Images

The [Dockerfile](./Dockerfile) provides an Alpine Linux-based image with Project
Kamisato binaries pre-installed.

You can use this image as a base to create your own package repository image, or
launch servers using Docker Compose.

These image files are published on the following image registries:

- [`hayao0819/kamisato`](https://hub.docker.com/r/hayao0819/kamisato)
- [`ghcr.io/hayao0819/kamisato`](https://github.com/Hayao0819/Kamisato/pkgs/container/kamisato)

For example configurations, see the [example](./example/) directory.

### Todo

- Implement inquiry sending
- Repository selection
- Support for multiple servers

## Special thanks

- <https://genshin.hoyoverse.com/ja/character/inazuma?char=0>
- [BrenekH/blinky](https://github.com/BrenekH/blinky)
