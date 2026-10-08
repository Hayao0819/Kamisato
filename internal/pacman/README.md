# Pacman package boundaries

These packages model stable pacman concepts rather than stages of a particular
command. They are shared by the products; product configuration, workflows,
flags, presentation, and command registration do not belong here.

| Package | Responsibility |
| --- | --- |
| `pkg` | Source/binary metadata, archives, artifact names, and source files |
| `depend` | Version constraints, dependency graphs, and AUR resolution |
| `repo` | Database artifacts, parsing, merge/diff, and mutation tools |
| `sign` | OpenPGP keys, trust verification, and detached signatures |
| `builder` | Backend contracts and host/project build configurations |
| `builder/{docker,devtools,bwrap}` | Concrete execution environments |
| `builder/factory` | Backend selection; separate from the contract |
| `builder/internal/*` | Helpers shared only by backend implementations |
| `host` | Host pacman/makepkg configuration and installed/sync databases |
| `nvcheck` | Upstream version sources and checking, not job scheduling |
| `hook` | Hook-file rendering, placement, and removal |
| `limits` | Shared package/signature size policies |

`repo`, `host`, and backend helpers may use `pkg`. Repository signing may use
`sign`. Other concepts stay independent. CLI hook construction and input handling
belong to `internal/cli/hook`, not `hook`.

External effects are implemented here as concrete adapters. Consumers inject a
backend, signer, database tool, queue, or request client where they need that
capability. There is no universal dependency container, and ordinary filesystem
operations are not turned into individual function fields. Network operations
accept the caller's context and client so cancellation and transport policy do
not disappear inside a shared library.

Keep a struct when it owns state, enforces a real invariant, names a meaningful
request/result, or implements a useful capability. Do not add wrapper types,
one-line forwarding layers, or a new package just to make directory names match.
For example, host and project build configurations intentionally remain separate:
project input must never gain control of host paths or daemon endpoints.

`TestPackageDependencies` guards these directions. A genuinely new concept may
extend the table and test deliberately; routine features should not require
another project-wide directory reshuffle.
