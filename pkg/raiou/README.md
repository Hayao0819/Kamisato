# Raiou

Raiou is a library for parsing ALPM metadata.

## Supported File Formats

- .BUILDINFO
- .PKGINFO
- desc
- .SRCINFO (powered by [go-srcinfo](https://github.com/Morganamilo/go-srcinfo))

## .SRCINFO compatibility layer

Raiou keeps go-srcinfo as the parser for section ordering, required fields,
architecture validation, split-package overrides, and line-oriented errors. It
adds a compatibility layer instead of exposing the upstream structs directly:

| Behavior | go-srcinfo v1.0.0 | Raiou |
| --- | --- | --- |
| Current makepkg `cksums` / `cksums_<arch>` | Ignored | Preserved in `CKSums` |
| Explicit empty split override | Internal NUL sentinel can escape | Exposed as an empty Go value |
| Fully resolved split packages | `SplitPackage(s)` can retain the sentinel for scalar/list fields | `SplitPackage(s)` resolves unset, replace, and extend semantics without sentinels |
| Architecture values | Flat `[]ArchString` | Grouped `ArchStrings`, with `ForArch` and `All` helpers |
| Serialized field names | No serialization tags | SRCINFO-compatible JSON/mapstructure names |
| Version formatting | Always includes `-pkgrel` | Handles partially populated metadata safely |

`SRCINFO.Packages` intentionally retains the raw pkgname sections, because
their values describe overrides rather than complete packages. Consumers that
need effective package metadata should use `SplitPackages` or `SplitPackage`.
