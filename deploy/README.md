# Deploying Kamisato

Two deployments: a single-VPS Docker Compose stack (`compose.yml`) and Google
Cloud Run. For per-service configuration, see the ayato and miko READMEs.

## Single VPS (Docker Compose)

```sh
export KAMISATO_BUILD_API_KEY="$(openssl rand -hex 32)"
export KAMISATO_PUBLISH_API_KEY="$(openssl rand -hex 32)"
export KAMISATO_REPO=aur
export KAMISATO_MAX_PACKAGE_SIZE=536870912 # bytes; shared by ayato and miko
export AYATO_AUTH_GITHUB_CLIENT_ID='<github-oauth-client-id>'
export AYATO_AUTH_GITHUB_CLIENT_SECRET='<github-oauth-client-secret>'
export AYATO_AUTH_PUBLIC_ORIGIN='https://repo.example.com'
export AYATO_AUTH_SESSION_SECRET="$(openssl rand -hex 32)"
export AYATO_AUTH_BOOTSTRAP_ADMIN_GITHUB_ID='<numeric-github-id>'
export AYATO_AUTH_TRUSTED_PROXIES='<tls-proxy-cidr>'
docker compose -f deploy/compose.yml up -d
```

The image must carry the combined `kamisato` binary; build one from the repo
`Dockerfile`.

`KAMISATO_MAX_PACKAGE_SIZE` is optional and defaults to 512 MiB. The compose
file writes the same value to both services' `max_size`, so miko rejects an
oversized build artifact before signing/upload and ayato applies the same limit
to every upload path.

An atomic split-package publish is separately bounded by
`KAMISATO_MAX_BATCH_PACKAGES` (default 16) and
`KAMISATO_MAX_BATCH_BYTES` (default 2 GiB). Every package must still satisfy
`max_size`, and every detached signature is capped at 16 MiB. The generated
JSON files are created under `umask 077` because they contain service keys.

The two service secrets are deliberately different. `KAMISATO_BUILD_API_KEY`
authenticates Ayato to Miko with `build:admin`; `KAMISATO_PUBLISH_API_KEY`
authenticates Miko to Ayato for `KAMISATO_REPO` publication and
`signer:register`. Neither key is a user credential and neither is accepted as
HTTP Basic. Users authenticate through GitHub OAuth and receive a Bearer token.

When GitHub login is enabled, ayato needs `auth.session_secret` (one or more
keys, each >= 32 bytes; the first signs, all verify, so a key can be rotated by
prepending a new one) and `auth.trusted_proxies` set to the fronting proxy's
CIDR. The per-IP rate-limit key is only trustworthy when that proxy is the sole
`X-Forwarded-For` setter; an empty `trusted_proxies` trusts none and falls back
to the direct peer.

Workers KV cannot atomically consume a rotating refresh token. Ayato therefore
fails startup when `auth.session_secret` is combined with `store.db_type=cfkv`;
use SQL (Cloud Run) or BadgerDB (single instance) for user/CLI authentication.

Presigned final-key upload is intentionally disabled until a staging-intent
protocol exists. Multipart uploads therefore traverse the ingress proxy; its
body-size limit must be at least `max_batch_bytes` plus multipart overhead, or
large packages must remain below the proxy's lower operational limit.

## GCP (Cloud Run)

The production Terraform for Cloud Run (the services, Secret Manager and R2
wiring, the `ayato-migrate` job, and the lumine Cloudflare Pages deploy) lives in
[alterlinux-terraform](https://github.com/FascodeNet/alterlinux-terraform); use
it as a reference. One caveat it can't paper over: Cloud Run can't nest
containers, so miko's `container` executor needs Cloud Build or a GCE VM.
