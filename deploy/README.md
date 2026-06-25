# Deploying Kamisato

Two deployments: a single-VPS Docker Compose stack (`compose.yml`) and a GCP
Cloud Run skeleton (`terraform/`). For per-service configuration, see the ayato
and miko READMEs.

## Single VPS (Docker Compose)

```sh
export KAMISATO_BUILD_API_KEY="$(openssl rand -hex 32)"
export AYATO_AUTH_USERNAME=ayato
export AYATO_AUTH_PASSWORD="$(openssl rand -hex 16)"
docker compose -f deploy/compose.yml up -d
```

The image must carry the combined `kamisato` binary; build one from the repo
`Dockerfile`.

When GitHub login is enabled, ayato needs `auth.session_secret` (one or more
keys, each >= 32 bytes; the first signs, all verify, so a key can be rotated by
prepending a new one) and `auth.trusted_proxies` set to the fronting proxy's
CIDR. The per-IP rate-limit key is only trustworthy when that proxy is the sole
`X-Forwarded-For` setter; an empty `trusted_proxies` trusts none and falls back
to the direct peer.

## GCP (Cloud Run skeleton)

`terraform/` is a skeleton, not a turnkey module. It provisions a service account
per service, the shared secret in Secret Manager, miko's ingress as `internal`,
and an IAM binding so only ayato's service account may invoke miko; set a VPC
connector in `vpc_access`. Cloud Run can't nest containers, so miko's `container`
executor needs Cloud Build or a GCE VM instead. Check the attribute values
against the provider docs before applying.
