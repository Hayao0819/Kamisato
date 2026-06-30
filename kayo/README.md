# Kayo

Kayo is a local, aurweb-compatible overlay you run on your own machine. Point an AUR
helper at it and it answers in place of the real AUR, federating three sources into
one catalog: trusted git overlays, other ayato instances, and the upstream AUR. Every
result passes through a local trust store before the helper sees it, so a hijacked or
freshly compromised AUR package can be held back from your install.

Run it as its own binary or as `kamisato kayo`. It listens on `127.0.0.1:10713` by
default:

```sh
kayo -c kayo_config.toml          # or: kamisato kayo -c kayo_config.toml
paru --aururl http://localhost:10713 --aurrpcurl 'http://localhost:10713/rpc?' <pkg>
```

## The trust model

aurweb authenticates the account that pushes a package, but it never verifies the
author or committer email on a commit, and real AUR repos routinely carry mixed,
forgeable emails. So kayo anchors trust to the **maintainer account** (the RPC
`Maintainer`), never to a git identity. Accounts are scoped per source, so `jguer` on
the AUR and `jguer` on some ayato are different principals.

Trust has two layers:

- **Resolution time** — who controls this package, and has that changed? An approval
  records the maintainer account at review time. If the current maintainer differs (a
  takeover, an adoption, or the package going orphaned), kayo flags it for review.
- **Build time** — is the content the same one you reviewed? `trust add` pins the
  reviewed commit by re-serving it from a local repo, so a later clone gets exactly
  those bytes even after the upstream source moves on.

Approving a package also vouches for its current maintainer. A vouch only sanctions a
future **handoff**: if an already-approved package is adopted by an account you vouch
for, that transfer is allowed without re-review. It never auto-trusts a brand-new
package. Overlays listed in the config are trusted outright.

## Commands

```sh
kayo audit <package|dir|git-url>   # static PKGBUILD scan + maintainer check
kayo trust add <package|git-url>   # review, pin the commit, record the approval
kayo trust list                    # list vouched maintainers and pinned packages
kayo trust rm <pkgbase>            # drop an approval (or --maintainer source/account)
kayo update <package|git-url>      # diff against the approved commit; --approve to re-pin
kayo verify [pkgname...]           # the install-time check the pacman hook runs
```

`audit` scans the PKGBUILD (and any `.install`) for behaviour real AUR supply-chain
attacks have used — piped shell, unexpected network fetches, obfuscation, checksum
skips — and exits non-zero on a high-severity finding. Add `--llm-advisory` for a
non-gating LLM triage pass.

`trust add` runs that same audit, refuses a high-severity package unless you pass
`--force`, pins the reviewed commit, and writes the approval. Pin a specific revision
with `--ref`. `update` shows what changed since the approval (maintainer, commit,
changed files), re-audits, and advances the pin with `--approve`.

## The pacman hook

`kayo verify` is the backstop at install time, for anything that reaches pacman
without going through the overlay. It resolves each target, skips official-repo
packages (not yours to gate), batches a single upstream lookup for the foreign ones,
and checks each against the trust store.

Install it as a PreTransaction hook. It writes to pacman's `HookDir` (usually
`/etc/pacman.d/hooks`), so it needs root:

```sh
sudo kayo hook install
sudo kayo hook uninstall
```

The hook fires on every install and upgrade and aborts the transaction when `verify`
fails.

## enforce vs warn

`enforce_mode` in the config decides what a needs-review verdict does:

- **warn** (default) — nothing is blocked. In the catalog, kayo annotates only a
  package that breaks an existing approval (such as a maintainer change); the hook
  prints a `REVIEW` line but lets the install proceed. Pass `--strict` to `verify` to
  fail anyway.
- **enforce** — unreviewed and changed-maintainer packages are dropped from the
  catalog the helper sees, and the pacman hook aborts the transaction. An upstream
  lookup that cannot complete fails closed rather than letting a package through
  unchecked.
