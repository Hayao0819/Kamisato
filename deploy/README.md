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

## GCP (Cloud Run skeleton)

`terraform/` is a skeleton, not a turnkey module. It provisions a service account
per service, the shared secret in Secret Manager, miko's ingress as `internal`,
and an IAM binding so only ayato's service account may invoke miko; set a VPC
connector in `vpc_access`. Cloud Run can't nest containers, so miko's `container`
executor needs Cloud Build or a GCE VM instead. Check the attribute values
against the provider docs before applying.
