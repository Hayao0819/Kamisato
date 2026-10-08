# Kayo

Kayo is a local aurweb-compatible overlay. You run it on your own machine, point
an AUR helper at it, and it answers RPC and git requests in place of the real
AUR. Behind that one endpoint it federates three sources: trusted git overlays,
other ayato instances, and the upstream AUR. A local trust policy governs these
results: enforce mode can hold back unreviewed packages and changed maintainers
before installation; warn mode reports them without blocking.

Run it as its own binary or as `kamisato kayo`. It listens on `127.0.0.1:10713`
by default.

```sh
kayo -c kayo_config.toml          # or: kamisato kayo -c kayo_config.toml
paru --aururl http://localhost:10713 --aurrpcurl 'http://localhost:10713/rpc?' <pkg>
```

## Trust model

aurweb authenticates the account that pushes a package, but it never checks the
author or committer email on a commit, and real AUR repos routinely carry mixed,
forgeable emails. So kayo anchors trust to the maintainer account (the RPC
`Maintainer` field) rather than any git identity. Accounts are scoped per
source, so `jguer` on the AUR and `jguer` on some ayato instance are two
different principals.

There are two checks, at two different times.

At resolution time kayo asks who controls the package now, and whether that
changed since you reviewed it. An approval records the maintainer account as it
was at review. If the current maintainer differs (a takeover, an adoption, an
orphaning), the package gets flagged for review.

At build time it asks whether the content is still what you reviewed. `trust
add` pins the reviewed commit and re-serves it from a local repo, so a later
clone gets those exact bytes even after the upstream source has moved on.

Approving a package also vouches for its current maintainer. That vouch only
covers a future handoff: if an already-approved package is adopted by an account
you vouch for, the transfer goes through without another review. It never
trusts a brand-new package on its own. Overlays listed in the config are trusted
outright.

## Commands

```sh
kayo audit <package|dir|git-url>   # static PKGBUILD scan + maintainer check
kayo trust add <package|dir|git-url> # review, pin, and record the approval
kayo trust list                    # list vouched maintainers and pinned packages
kayo trust rm <pkgbase>            # drop an approval (or --maintainer source/account)
kayo update <package|dir|git-url>  # show the diff; --approve to re-pin
kayo verify [pkgname...]           # the install-time check the pacman hook runs
```

`audit` scans the PKGBUILD and any `.install` for behaviour real AUR
supply-chain attacks have used: piped shell, unexpected network fetches,
obfuscation, skipped checksums. It exits non-zero on a high-severity finding.
`--llm-advisory` adds a non-gating LLM triage pass.

`trust add` runs that same audit, refuses a high-severity package unless you
pass `--force`, then pins the reviewed commit and writes the approval. Use
`--ref` to pin a specific revision. `update` shows what changed since the
approval (maintainer, commit, files), re-audits, and advances the pin when you
pass `--approve`.

Package names follow the same source priority as resolution and verification:
configured overlays, federated ayato sources, then AUR. An overlay review starts
from its configured revision, not a fresh remote HEAD. Approvals keep the selected
source's namespace and maintainer account, including for split packages.

Git URLs supplied by a federated catalog use the strict clone transport: local
files, private/loopback destinations, plaintext HTTP, and HTTP redirects are
rejected. A catalog signature authenticates its publisher, not permission to read
the reviewer's local files or contact internal services. Explicit directory/URL
targets and operator-configured overlays still support local repositories.

Approving a local Git directory reviews a temporary snapshot of the selected
commit. Without `--ref`, tracked changes must be committed first; with an explicit
ref, that revision is selected instead. Untracked files are not part of a pin.
The ordinary audit and non-approving update still inspect the working directory.

`trust whitelist add <pkgbase>` is a separate escape hatch: it skips review and
maintainer-change checks and pins no commit. It is not equivalent to `trust add`.

## Pacman hook

`kayo verify` is the backstop at install time, for packages that reach pacman
without going through the overlay. It resolves each target, skips official-repo
packages that aren't yours to gate, batches one upstream lookup for the foreign
ones, and checks each against the trust store.

Install it as a PreTransaction hook. It writes into pacman's `HookDir` (usually
`/etc/pacman.d/hooks`), so it needs root.

```sh
sudo kayo hook install
sudo kayo hook uninstall
```

The hook fires on every install and upgrade, and aborts the transaction when
`verify` fails.

## enforce vs warn

`enforce_mode` in the config decides what a needs-review verdict actually does.

Under `warn` (the default) nothing is blocked. In the catalog kayo annotates
only a package that breaks an existing approval, such as a maintainer change;
the hook prints a `REVIEW` line and lets the install continue. Pass `--strict`
to `verify` to make it fail anyway.

Under `enforce`, unreviewed and changed-maintainer packages drop out of the
catalog the helper sees, and the pacman hook aborts the transaction. An upstream
lookup that can't complete fails closed instead of letting a package through
unchecked.
