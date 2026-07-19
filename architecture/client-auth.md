# Client and authentication architecture

This document is the contract for in-repository calls between Ayaka, Ayato,
Miko, Thoma, Lumine, CI publishers, and the signing service. The HTTP surface is
still `/api/unstable`; the shared Go client therefore remains under `internal/`.

## Call graph

```text
Ayaka / Thoma ── Bearer (user session) ──> Ayato
Lumine         ── session cookie/Bearer ─> Ayato
CI publisher   ── X-API-Key or OIDC ─────> Ayato native publish API
legacy Blinky  ── Basic ─────────────────> Ayato /blinky only

Ayato          ── named X-API-Key ───────> Miko build API
Miko           ── named X-API-Key ───────> Ayato publish/signer registration
Miko worker    ── named X-API-Key ───────> remote signing service
Kayo           ── no credential ────────> Ayato public catalog/key API
```

An end-user credential must never cross the Ayato-to-Miko proxy. The proxy
removes `Authorization`, `Cookie`, and a caller-supplied `X-API-Key`, then adds
only Ayato's configured Miko key.

## Authentication matrix

| Caller | Callee and surface | Credential | Authorization |
| --- | --- | --- | --- |
| Ayaka, Thoma | Ayato `/api/unstable` | `Authorization: Bearer` | admin allowlist; short-lived CLI access token |
| Lumine | Ayato `/api/unstable` | session cookie or Bearer | admin allowlist |
| Ayato | Miko `/api/unstable` | `X-API-Key` | named Miko key, normally `build:admin` |
| Direct Thoma | Miko `/api/unstable` | `X-API-Key` | least-privilege build scopes and per-owner jobs |
| Miko | Ayato native publish | `X-API-Key` | `publish_repos` for the destination repo |
| Miko startup | Ayato signer registration | `X-API-Key` | `signer:register` |
| Miko worker | Remote signer | `X-API-Key` | `sign` |
| Kayo | Ayato public catalog/key API | none | read-only public surface |
| GitHub Actions | Ayato native publish | GitHub OIDC Bearer JWT | repository id/ref/audience and destination repo |
| Legacy Blinky | Ayato `/blinky` | HTTP Basic with CLI token in password | admin allowlist; compatibility only |
| Old Miko (migration only) | Ayato signer registration | HTTP Basic with CLI token in password | only when `auth.allow_legacy_signer_basic=true` |

Basic authentication is not accepted by native Ayato routes, the Miko build
API, or the remote signer. Signer registration has one explicit, default-off
Ayato-first rollout bridge for old Miko replicas; a presented `X-API-Key` never
downgrades to Basic. New code must not construct a Blinky client.

## Shared client boundary

`internal/client` owns URL construction, credentials, HTTP policy, wire types,
and error parsing. Callers choose one capability-specific constructor:

- `NewAyato(base, BearerTokenSource)` for user-facing Ayato operations.
- `NewMiko(base, apiKey)` for direct build-server operations.
- `NewPublisher(base, apiKey)` for workload publish and signer registration.
- `NewSigner(base, apiKey)` for the isolated detach-sign operation.
- `NewCatalog(base)` for Kayo's credential-free public reads.

The smaller `Publisher` type intentionally cannot invoke admin or build
operations. The former function-based `internal/buildclient` facade was removed
after every caller moved to these capability clients. `internal/blinkyutils`
retains only the released Blinky `servers.json` representation and translates it
into Kamisato-owned types; upstream concrete types and type aliases never cross
that boundary. `internal/serverstore` owns endpoint selection, keyring state,
token rotation, and atomic persistence, and consumes only the owned registry
codec.

### Transport invariants

- Parse the base URL once and retain a reverse-proxy path prefix.
- Reject userinfo, query strings, fragments, and non-HTTP schemes in base URLs.
- Escape every dynamic path segment as data.
- Use an instance `http.Client`; do not mutate global HTTP state.
- Never attach credentials to public repository-object downloads.
- User Bearer and refresh flows require HTTPS; plaintext is accepted only on a
  loopback endpoint for local development. Service HTTP is restricted to an
  explicitly trusted internal network such as the sealed Compose bridge.
- Never follow a redirect from a credentialed request. Strip all credentials
  before any allowed public redirect.
- Retry replay-safe reads only. Do not retry submit, cancel, upload, delete,
  token rotation, or other mutations at the transport layer.
- Parse Ayato `{message,reason}`, Miko `{error}`, and plain-text failures into a
  common response error without making the servers share an error domain type.

### Refresh sequencing

Ayato marks only an expired, otherwise valid CLI access token with
`X-Access-Token-Expired: 1`. An ordinary `401` is never treated as refreshable.
The client then:

1. acquires the per-server process/file refresh lock;
2. reloads the stored access token in case another process already rotated it;
3. exchanges the refresh token once;
4. persists the new access/refresh pair; and
5. reconstructs the entire client operation once.

The HTTP transport never replays an arbitrary request body. Multipart uploads
open the files and build a fresh stream for the reconstructed operation.

Server-side refresh consumes the old refresh JTI using the KV backend's atomic
`Add` capability. If the configured backend cannot provide an atomic insert,
refresh fails closed with `503`; it must not fall back to `Get` followed by
`Set`. BadgerDB and SQL provide the required primitive. Workers KV currently
does not, so Ayato rejects `cfkv` plus `auth.session_secret` at startup instead
of accepting a login that fails only after access-token expiry.

The client passes the access token that received the expiry marker into
`RefreshIfCurrent`; concurrent 401s coalesce instead of repeatedly rotating the
new credential. It probes the existing refresh-token keyring entry before the
server consumes it, and saving the replacement refresh token is checked rather
than treated as best-effort.

The access/refresh pair is loaded as one `CredentialSnapshot` under the global
credential lock. Network I/O then runs without holding the lock, and the result
is installed with a revision compare-and-save. A concurrent login, logout, or
static-token replacement wins over a stale refresh response. Revoke follows the
same rule: it sends the exact snapshotted pair and compare-clears only that
revision, so a login completed while revoke is in flight cannot be erased.

Every newly issued access/refresh pair carries one stable session-family id.
Rotation preserves that family and the original refresh expiry; revocation
denylists both presented JTIs and the family for the remaining refresh lifetime.
Consequently, revoking an ancestor also invalidates a descendant minted by a
concurrent refresh. A legacy refresh token without a family adopts its own JTI
as the family on first rotation, preserving this guarantee across migration.

Log-stream tokens, device codes, PKCE codes, and refresh JTIs all use the same
security rule: a shared-KV atomic `Add` creates a remaining-TTL spent marker,
and exactly one caller wins. Source-record deletion is only garbage collection.
There is no `Get`-then-`Delete` or `Get`-then-`Set` fallback. Denylist misses are
distinguished from backend failures; a failure to check revocation returns `503`
instead of authenticating a possibly revoked Bearer token.

## Credential storage compatibility

`internal/blinkyutils` owns the released Blinky database path, JSON schema, and
raw server URL map key. `internal/serverstore` deliberately retains the
remaining released credential identifiers:

- keyring service `kamisato-ayato`;
- the existing access-token key; and
- the existing NUL-suffixed refresh-token key.

Ayaka additionally writes `blinky-cli/credential-state.json`, an atomic,
mode-only sidecar (`keyring`, `file`, or `none`; never a secret). An absent entry
retains the legacy lookup order. Once written, this mode is authoritative, so a
stale secret that could not be deleted while Secret Service was unavailable
cannot reappear after logout or static-token replacement. This preserves the
file fallback on SSH/headless hosts while refresh tokens remain keyring-only.

Every mutation holds one process mutex and one cross-process `flock` across the
complete `servers.json` + keyring + sidecar transition. Both JSON files use a
mode-0600 temporary file, file `fsync`, atomic rename, and parent-directory
`fsync`. A replacement first durably commits `{access:none,refresh:none}` and
only enables the completed access/refresh pair in its final sidecar commit, so a
crash cannot expose a mixed pair. A corrupt or unreadable sidecar fails closed
for both sources. Remote revoke uses compare-and-clear: if another process logs
in while the network request is in flight, the newly stored pair is retained.

The legacy `Password` field at this persistence boundary contains a native
Ayato Bearer token, not a Basic password. CLI help uses `token` terminology.
`server add --password-stdin` remains only as a deprecated alias for
`--token-stdin`; `server login` is the preferred OAuth flow.

## Miko named keys and job ownership

Miko's `auth.api_keys` entries have a unique key `name`, stable `principal`,
secret, and scopes. `principal` defaults to `name`; two unique key names may
share one principal during rotation and retain access to existing jobs:

- `build:submit`
- `build:read`
- `build:cancel`
- `build:admin`
- `sign` (remote signer)
- `*` (migration/emergency use only)

`build:admin` implies the build submit/read/cancel capabilities. The submitting
stable principal is injected by middleware and persisted as the job owner; JSON
request input cannot choose it. Non-admin keys can list, read, cancel, stream,
or download only their own jobs. Cross-owner access returns `404` to avoid an
existence oracle. Old top-level `api_keys` values temporarily map to synthetic
legacy names with full scope. Scope denials log principal and key id without the
secret, and legacy-key use emits a migration warning.

## Wire contracts

`internal/protocol` contains the Miko request/response DTOs used by servers,
Go clients, and Tygo. It is separate from `miko/domain`, which contains mutable
execution and persistence state. Public job JSON excludes the request body,
artifact directory, and owner authorization metadata.

CI regenerates TypeScript from `internal/protocol` and fails if generation
changes the worktree. This makes a Go wire change and its frontend projection a
single reviewed change.

## Package publishing

Native publication uses `POST /api/unstable/repos/:repo/packages` with a
validated multipart body. Miko preflights and signs the entire split-package
build result, then sends it in one request. Ayato validates the complete batch
before mutation, including an exact match between each basename and its
`.PKGINFO` tuple (`pkgname`, normalized `pkgver`, and `arch`), and rejects
duplicate object names in a batch.

Package and signature object names are immutable. Storage first compares the
candidate SHA-256 with any existing object: identical bytes are an idempotent
reuse, while different bytes under the same key are a conflict. Creation uses
create-only CAS. S3/R2 uses object preconditions; localfs uses a per-object
cross-process `flock`, a temporary file plus `fsync`/atomic rename, and the live
SHA-256 as its version token. The S3 SDK retryer is disabled for each conditional
PUT: a committed request whose response was lost must remain an ambiguous error,
not be replayed with a stale `If-Match` and collapsed into a misleading `412`.
Failed publications do not eagerly delete these objects because that could
delete a concurrent winner; unreferenced immutable objects are considered only
by the orphan reconciler. Identical reuse renews the object's modification
lease. On localfs, upload, promotion, rollback, and orphan collection also share
one repository-wide cross-process publication `flock`. GC takes that lock before
its reference snapshot and retains it through deletion, closing the
object-before-DB visibility window even when the configured age is zero. It
then revalidates the captured version/mtime and cutoff under the per-object lock
before deletion. Negative ages are rejected. A backend that cannot provide an
atomic freshness-checked delete (currently the S3/R2 adapter) reports the
candidate but does not delete it online; operators must collect those objects
during a quiesced/offline maintenance window.

Ayato captures every replaced package/signature and name mapping, and updates
each architecture with one DB CAS. The expected old version and filename are
revalidated inside every CAS retry, so a stale publisher cannot downgrade or
remove a concurrent winner. A retry whose canonical DB already contains this
writer's intended version/file is idempotent. If a later architecture, derived
artifact, or name write fails, Ayato compensates only while its just-published
version/filename still matches; otherwise it preserves the newer writer's blobs
and metadata. Captured versions and name mappings are restored when the
conditional compensation still owns them.
Before restoring a prior canonical DB entry, compensation recreates or renews
the captured old package and signature objects. Thus a collector cannot remove
an old, temporarily unreferenced object between the failed upgrade and its
canonical restoration.

`<repo>.db.tar.gz` is the canonical package set. After its CAS succeeds, the
`.files` archive and optional signatures are reconciled against their current
ETags only while the live canonical bytes still match this writer; a canonical
change forces a fresh read/reapply instead of treating a derived-object conflict
as evidence of a newer state. Package-name lookup is DB-first and its KV entry is
only a cache, so a partial cache restore cannot redirect reads away from the
canonical database.
Once any canonical CAS has succeeded, every later failure is returned as a
typed canonical-commit error even if another writer supersedes it during a
retry. A non-precondition error from the canonical write is also treated as
outcome-ambiguous, since a backend can commit and then lose the response; the
same typed path forces idempotent retry and conservative compensation.

Upload compensation, startup initialization, and signature backfill all invoke
the same architecture-level `ReconcileDB` operation. It snapshots canonical,
accepts file lists only from `.files` entries whose directory and raw `desc`
exactly match canonical, and reads immutable package blobs only for missing or
mismatched entries. It writes derived artifacts conditionally while continually
checking that canonical still matches the snapshot, and never writes canonical
itself. This makes repair idempotent and prevents a stale `.files` archive from
resurrecting a deleted package or downgrading a current one. A corrupt or
truncated `.files` archive is treated as disposable: repair removes the local
copy, reloads canonical alone, materializes each required immutable package
object, and regenerates the derived archive without changing canonical bytes.
Package signatures are matched by the
`<package>.sig` convention. The old presign protocol wrote to the final object
key before validation. Its service and blob-store capabilities have been
removed; compatibility endpoints return a fixed `501` so older clients can
fall back safely. Reintroducing direct upload requires an opaque staging-intent
protocol that validates and atomically promotes an object. Atomic batches are
bounded independently: 16 packages and 2 GiB of file data by default, plus the
per-package `max_size` and a 16 MiB signature cap. Multipart traverses the
ingress, whose body limit must match this policy.

This is failure compensation across independent blob/KV objects, not a claim
that the backing stores provide a cross-object transaction. A future strict
all-architecture commit should publish immutable generations and CAS one
manifest pointer. Rollback failures are logged as operational errors; the
current implementation's regression tests cover name-store failure after DB
commit and failure on the second architecture.

Native deletion supports both one architecture and all architectures. New
callers do not fall back to Blinky Basic for deletion.

## Rollout and rollback

The auth endpoints are a homogeneous-fleet boundary. Do not round-robin old and
new Ayato binaries for `/auth/refresh`, `/auth/device/token`, or one-time log
streams: old binaries do not understand the new spent namespaces. Use a
blue/green pool switch (or backport atomic consumption first), stop routing to
the old pool, and drain it before the new pool accepts auth traffic.

1. Pre-stage direction-specific keys in secret/config management. Before
   deploying new Ayato, remove the obsolete Ayato-side `auth.username` and
   `auth.password`; the new binary rejects them to prevent a silent false sense
   of protection. Keep old Miko-side `ayato.username/password` only while old
   Miko replicas still need their rollback path.
2. Deploy Ayato first as a separate homogeneous pool with native
   multipart/delete routes and scoped service keys, switch traffic atomically,
   and drain the old Ayato pool. If old Miko replicas register local signers,
   temporarily set
   `auth.allow_legacy_signer_basic=true`.
3. Deploy Miko accepting named and legacy inbound keys, and using the new Ayato
   publish/registration `X-API-Key`. Local-signer startup fails readiness if its
   certificate cannot be registered.
4. Deploy the Ayato proxy with a named `build:admin` Miko key, then deploy Ayaka
   and Thoma using `internal/client` and `serverstore`.
5. Observe key-id/principal audit logs, drain old replicas, remove legacy Miko
   keys, and disable `auth.allow_legacy_signer_basic`.
6. After the Miko rollback window closes, remove its old
   `ayato.username/password`. Deprecated Ayato-side fields remain only to reject
   them with an actionable startup error.
7. Remove `/blinky` only in a separately announced compatibility release.

Rollback keeps the stored server schema and keyring identifiers intact. The
supported mixed-version window is Ayato-first for Miko only: Miko may be rolled back while
Ayato's explicit signer bridge remains enabled. New Miko is not promised to
work with old Ayato; roll Ayato forward again before reverting Miko. Do not
remove a legacy Miko key until every Ayato replica has the named key, and drain
old replicas before revoking it.

Rolling Ayato back across the atomic-consume boundary is not supported after the
new pool has accepted auth traffic unless the old release received the spent
marker backport. Likewise, old Ayaka ignores `credential-state.json`; after the
sidecar has been written, downgrade is unsupported unless a new Ayaka first
logs out with a working OS keyring (so physical deletion succeeds), the operator
verifies/removes both `kamisato-ayato` keyring entries, and the downgraded client
then performs a fresh login. This prevents an old binary from reactivating a
sidecar-tombstoned secret.
