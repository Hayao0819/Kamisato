# Deploying Kamisato

Two deployments: a single-VPS Docker Compose stack (`compose.yml`) and Google
Cloud Run. For per-service configuration, see the ayato and miko READMEs.

## Single VPS (Docker Compose)

```sh
export KAMISATO_BUILD_API_KEY="$(openssl rand -hex 32)"
export KAMISATO_MAX_PACKAGE_SIZE=536870912 # bytes; shared by ayato and miko
export AYATO_AUTH_USERNAME=ayato
export AYATO_AUTH_PASSWORD="$(openssl rand -hex 16)"
docker compose -f deploy/compose.yml up -d
```

The image must carry the combined `kamisato` binary; build one from the repo
`Dockerfile`.

`KAMISATO_MAX_PACKAGE_SIZE` is optional and defaults to 512 MiB. The compose
file writes the same value to both services' `max_size`, so miko rejects an
oversized build artifact before signing/upload and ayato applies the same limit
to every upload path.

When GitHub login is enabled, ayato needs `auth.session_secret` (one or more
keys, each >= 32 bytes; the first signs, all verify, so a key can be rotated by
prepending a new one) and `auth.trusted_proxies` set to the fronting proxy's
CIDR. The per-IP rate-limit key is only trustworthy when that proxy is the sole
`X-Forwarded-For` setter; an empty `trusted_proxies` trusts none and falls back
to the direct peer.

## GCP (Cloud Run)

The production Terraform for Cloud Run (the services, Secret Manager and R2
wiring, the `ayato-migrate` job, and the lumine Cloudflare Pages deploy) lives in
[alterlinux-terraform](https://github.com/FascodeNet/alterlinux-terraform); use
it as a reference. One caveat it can't paper over: Cloud Run can't nest
containers, so miko's `container` executor needs Cloud Build or a GCE VM.
