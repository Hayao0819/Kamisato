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

## Package layout

Each component keeps its implementation under its own directory. Package names
describe the same responsibility across components; a component only adds the
layers it needs.

| Package | Responsibility |
| --- | --- |
| `cmd` | Cobra commands and command-local input, execution, and output |
| `cmd/<family>/internal/<role>` | Family-scoped CLI helpers |
| `cmd/internal/<role>` | Component-scoped CLI helpers |
| `config` | Configuration types, loading, and validation |
| `server` | Server dependency assembly and startup/shutdown |
| `service` | Build, repository, and trust workflows |
| `domain` | Component-owned models and policies |
| `client` | HTTP client and wire types for callers of the component |
| `protocol` | Wire contracts exchanged between components |

These are responsibilities, not a mandatory stack of directories. Keep existing
specific roles such as `handler`, `repository`, `source`, and `trust` when they
describe the code more precisely. Do not manufacture `domain`, a service object,
or an interface just to complete a layer diagram.

`cmd/root.go` exports `RootCmd()`. Every subcommand, including a leaf, has its own
directory following the command tree,
uses `<command>cmd` as its Go package name, and exports `Cmd()` for its parent.
Hyphenated verbs keep their CLI spelling in directory names, remove hyphens in
Go package names, and use underscores in Go filenames (`set-default/set_default.go`,
`package setdefaultcmd`).
For example, `ayaka server admin add` belongs in
`ayaka/cmd/server/admin/add/add.go` (`package addcmd`, `func Cmd()`).
Keep single-command helpers beside that command. Share helpers at the narrowest
scope that needs them: a command family's `internal` directory, then the
component's `cmd/internal`. Give each helper package a concrete responsibility,
not a catch-all `common`, `utils`, `app`, or `cli` layer. The Go `internal` rule
enforces this CLI-only scope; helper package names do not have the `cmd` suffix.

Parent commands register children; children never import parents or siblings.
Extract shared behavior instead of calling another command's execution function.
Command constructors only declare flags and assemble the command tree. They do
not open files, resolve credentials, contact servers, or start services. Group
commands can declare inherited flags and hooks, but do not own a child command's
implementation.

The command adapter validates arguments, loads configuration, resolves credentials
and clients, and calls its workflow. Nontrivial execution functions take parsed
values, a context, and the operations they actually use rather than a Cobra
command, a flag set, or a whole application object. Short, single-purpose adapters
need not be split into `options.go`, `run.go`, and `output.go`. Repository discovery
and authenticated clients remain lazy so help/version and dry runs do not load
unrelated configuration or request credentials.

Use ordinary functions for stateless behavior. Add a struct for meaningful input,
output, policy, or state/resource ownership, not to replace a function call with
one forwarding method. `Options` is useful when it groups several related inputs;
it is not required for every command. Do not mirror structs between the CLI
and a service when the same operation input already fits both.

Dependency injection follows the same rule everywhere: inject real external
boundaries (HTTP clients, authenticated API clients, stores, builders, signers,
or a workflow operation). An interface belongs to its consumer and describes
only the methods it needs; a concrete client or a single function is enough when
that is the simpler seam. Private test constructors are fine. Public `Cmd()` and
`RootCmd()` do not take recursive `Dependencies` trees, and we do not wrap every
`os`, clock, or formatting function in a configurable field. Libraries may offer
a convenient default constructor alongside explicit client/backend options;
defaults must not hide configuration or credential discovery in a workflow.
The code that opens a closable resource owns its cleanup, and contexts must reach
network requests, jobs, and subprocesses.

CLI-independent workflows belong in `service`. Services do not depend on
`server`, `cmd`, or Cobra. Return structured decisions/results and errors; tables,
colors, JSON rendering, and CLI-specific messages belong to commands. Streaming
build logs and execution progress can use explicit writers. `server` assembles
the resources used by a running server and manages its lifetime: maintenance
commands must not initialize an unrelated server to use a store or a service.
`config` owns decoding, validation, and pure mapping of settings, not resource
construction. Components communicate
through `client` or `protocol` packages rather than importing another component's
server implementation.

The root `internal` directory contains shared Kamisato infrastructure: CLI and
configuration helpers, HTTP transport/lifecycle, filesystem and Git operations,
and common pacman building, metadata, repository, and signing operations.
Component-specific source repository management and keyring assembly live under
Ayaka; Ayato owns its clients, catalog protocol, and storage encryption.
`pkg/raiou` and `pkg/aurweb` are the standalone reusable libraries.
`internal` expresses import visibility, not a requirement that code be shared:
component- and command-local internal packages are appropriate too.
`internal/cli` is shared CLI infrastructure, not a second product CLI layer.
`internal/pacman` is organized by stable capabilities, not by the callers'
commands; see its [scope and dependency rules](./internal/pacman/README.md).

Preserve flags, arguments, configuration keys, wire formats, JSON fields, and
exit behavior when refactoring. Use the command's input/output streams, keep
diagnostics on stderr, print an error once, and preserve non-interactive behavior.
Thoma deliberately keeps makepkg's raw argument order and passthrough behavior:
the shared rules do not require a conventional subcommand tree for a shim.

The root structure tests check package names, command placement, and dependency
boundaries so subsequent additions follow these conventions. Test command
registration/help laziness and CLI contracts separately from workflow policies,
and use real temporary files/local HTTP fixtures where they test the relevant
boundary better than mocks.

### Design references

The conventions above are a Kamisato-specific synthesis, not a claim that Go or
Cobra requires one layout:

- [GitHub CLI's command development guide](https://github.com/cli/cli/blob/trunk/docs/command-development.md)
  provides the closest match for independent leaf-command directories,
  command-family sharing, lazy repository discovery, and script-facing contracts.
  We do not adopt its global Factory or require a dependency struct per command.
- [Docker CLI's container run command](https://github.com/docker/cli/blob/master/cli/command/container/run.go)
  demonstrates parsed operation options and a real client/stream boundary.
  Its command grouping is not copied: Kamisato consistently uses leaf directories.
- [Helm's install command](https://github.com/helm/helm/blob/main/pkg/cmd/install.go)
  separates command wiring, an actual install operation, and result rendering.
  Operation objects are useful when they own meaningful configuration and behavior,
  not as compulsory wrappers for small functions.
- [Argo CD's CLI](https://github.com/argoproj/argo-cd/blob/master/cmd/argocd/commands/root.go)
  and [server entrypoint](https://github.com/argoproj/argo-cd/blob/master/cmd/argocd-server/commands/argocd_server.go)
  are relevant to independently runnable clients and servers in one project.
  Kamisato keeps its existing component ownership rather than introducing another
  parallel `cmd/<binary>` hierarchy.
- [Go's package naming guidance](https://go.dev/blog/package-names)
  supports concrete, concise package names instead of vague utility buckets.

The stable contract is ownership and dependency direction. Changes in one
backend or command should not require reshuffling unrelated directories; new
roles are added when the code genuinely needs them, not in anticipation of an
imagined universal framework.

Command groups describe distinct operator-facing resources or tasks, not these
implementation layers. The current Ayaka tree separates local sources (`src`),
published packages (`repo`), remote jobs (`miko`), private signing keys (`key`),
and distributed keyring packages (`keyring`). Local `plan` resolves build order;
`ci plan` produces source/repository-difference plans for automation. Keep these
paths and the existing hidden compatibility commands unless a concrete usability
problem justifies a separately reviewed CLI migration. Internal package cleanup
alone is not a reason to rename user commands.

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
