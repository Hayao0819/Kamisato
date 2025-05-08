# Kamisato

Kamisato is a set of tools for managing your pacman repository.

## Ayaka

Ayaka is a command line front end. It builds packages locally and deploys them
to Ayato or Blinky.

### Features

- Build all packages within chroot environment
- Upload built binary to blinkyd server
- Ayaka works as client of blinkyd

### Todo

- Mirroring repository to another server
- Signing packages with GPG key

## Ayato

Ayato is a Blinky compatible backend.It handles package hosting with automatic
database updates.

### Todo

- Implement basic features

## Lumine

Lumine is a web frontend for Ayato, built using Next.js, that displays server
status and searches for packages.

### Todo

Lumine is waiting for Ayato's implementation.
