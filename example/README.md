# Examples

Two ways to try kamisato, for two different goals.

- **`native.sh`** — the simplest taste, on Arch Linux, with no setup.
- **`compose.yml`** — the full stack in Docker, configurable, to explore most of
  what kamisato does.

## Native (Arch Linux) — `./native.sh`

One command, no Docker, no config to write:

```sh
./example/native.sh
```

It builds the `kamisato` binary, starts **ayato** (the repo server) and **lumine**
(the web UI), builds `dummypkg` with **makepkg**, publishes it with a demo API
key, and serves it as a real pacman repository. Everything runs from a throwaway
temp dir and is removed when you press Ctrl-C.

It needs only an Arch box with `go`, `base-devel` (makepkg/fakeroot), `pacman`
(repo-add) and `curl` — the package is built and indexed with the host's own Arch
tooling, no container involved. The script prints the web UI and API URLs and the
two lines to add to `/etc/pacman.conf` so you can `pacman -S dummypkg` from it.

This is the early-kamisato spirit: native binaries, one short script, easy to read.

## Docker (full stack) — `compose.yml`

The Docker stack trades that native simplicity for freedom — you configure every
component and run pieces the native demo leaves out. It brings up:

| Service | Port | Role |
|---|---|---|
| **ayato** | 8080 | repo server + aurweb-compatible host (`/rpc`, signed catalog) |
| **lumine** | 3000 | web console, served same-origin in front of ayato |
| **miko** | — | build server (builds packages in containers via the dind daemon) |
| **kayo** | 10713 | local aurweb overlay/router, federating ayato's catalog |
| **dind** | — | Docker-in-Docker daemon miko builds inside |

```sh
docker compose -f example/compose.yml up -d --build
```

### Browse and publish (works out of the box)

Open the console at <http://localhost:3000>, then publish the prebuilt `dummypkg`
with the example CI key — the upload route accepts a CI API key, so no login is
needed:

```sh
curl -X POST -H "X-API-Key: example-ci-key" \
  -F package=@example/ayaka/out/myrepo/x86_64/dummypkg-1.0.0-1-any.pkg.tar.zst \
  http://localhost:8080/api/unstable/repos/myrepo/packages

curl http://localhost:8080/api/unstable/repos/myrepo/arches/x86_64/packages   # now listed
```

ayato also answers aurweb's RPC and serves a signed catalog for kayo:

```sh
curl "http://localhost:8080/rpc/v5/suggest/dummy"     # aurweb-compatible RPC
curl http://localhost:8080/api/unstable/aur/pubkey    # catalog signing key
```

### kayo overlay (aurweb federation)

**kayo** runs as a local aurweb at <http://localhost:10713>. On startup it
federates the example ayato: it fetches ayato's signed catalog and pins ayato's
key on first use, ranking ayato above the real AUR and falling through to the AUR
for everything else. Point an AUR helper at it:

```sh
# paru.conf:  AurUrl = http://localhost:10713
paru --aururl http://localhost:10713 <pkg>
```

ayato's federated catalog stays empty until an admin registers AUR sources on it
(below), but the federation, catalog signing and key-pinning all run as soon as
the stack is up — `docker compose logs kayo` shows the pin on first sync.

### Unlock builds, admin and web login (configure GitHub OAuth)

Submitting builds to miko, managing the admin allowlist, registering AUR sources,
and logging into the web console all require authentication, which ships disabled.
To turn it on, give ayato a GitHub OAuth app in `config/ayato_config.json`:

```jsonc
"auth": {
  "github": { "client_id": "…", "client_secret": "…" },
  "public_origin": "http://localhost:3000",
  "session_secret": ["<at least 32 bytes>"],
  "trusted_proxies": ["172.16.0.0/12"],
  "bootstrap_admin_github_id": <your numeric GitHub id>
}
```

Then logging in mints a token and lets you drive miko from the host:

```sh
ayaka server login http://localhost:8080
cd example/ayaka
ayaka miko build myrepo dummypkg --server http://localhost:8080
ayaka miko logs <job-id> --server http://localhost:8080
```

### Notes

- `config/` holds the ayato, miko and kayo configs. ayato↔miko share the API key
  `example-build-key`; the upload CI key is `example-ci-key`.
- `auth.trusted_proxies` lists who may set `X-Forwarded-*` so the per-IP rate
  limit keys off the real client; here it is the compose subnet `172.16.0.0/12`.
- `AYATO_AUR_SIGNING_SEED` in `compose.yml` is a throwaway demo seed so ayato
  signs its catalog. Generate your own with `kamisato ayato aur keygen`.

Tear down with `docker compose -f example/compose.yml down -v`.
