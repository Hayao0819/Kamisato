# Deploying the closed Kamisato topology

Two deployments, same shape: a single-VPS Docker Compose stack (`../compose.yml`) and a GCP Cloud Run skeleton (`terraform/`).

## Shape

Clients (lumine, ayaka) talk only to ayato. ayato owns the repository and packages and never runs builds; it forwards build requests over a closed internal network to miko, adding a shared API key. miko owns the jobs and their state. ayato keeps no job state and just relays.

Clients never reach miko, enforced two ways:

- **Network isolation** — miko has no route out. Compose: the `internal: true` network. Cloud Run: `ingress = internal`.
- **API key** — miko rejects any request without the shared key; ayato adds it on every call. This is the second wall if the first is misconfigured.

Both walls exist because miko mounts the host Docker socket to build (Docker-out-of-Docker). A mounted `docker.sock` is root on the host, so reaching miko and submitting a build means owning the machine. Keep it reachable only through ayato.

## Client API (ayato, never miko)

- `POST /api/unstable/build` — Basic auth, returns `202 {"job_id": id}`
- `GET /api/unstable/jobs` — list
- `GET /api/unstable/jobs/:id` — status
- `GET /api/unstable/jobs/:id/logs` — live logs (SSE)

Body: `{ repo, arch, git:{url,ref,subdir} | pkgbuild, files:{name:contents}, install_pkgs:[], gpg_key }`.

## Env vars

| Variable | Service | Meaning |
| --- | --- | --- |
| `AYATO_MIKO_URL` | ayato | miko internal URL, e.g. `http://miko:8081` |
| `AYATO_MIKO_API_KEY` | ayato | key ayato sends to miko |
| `MIKO_API_KEYS` | miko | accepted inbound keys |

`AYATO_MIKO_API_KEY` and one entry of `MIKO_API_KEYS` are the same secret — generate it once.

**Caveat:** the config loader splits env vars on `_`, so it can't reach the `api_key` / `api_keys` tags (the underscore collides with the delimiter). Set these two through a config file (`-c`), not a bare env var. The Compose stack renders that file at startup. For miko, `miko apikey generate` writes a fresh key straight into its JSON config.

## Single VPS — Docker Compose

`../compose.yml` runs both on one host. ayato joins `edge` (for a reverse proxy) and `internal` (to reach miko) and publishes only on `127.0.0.1`; miko joins `internal` alone and publishes nothing.

The `127.0.0.1` binding matters: Docker's port rules run ahead of the host firewall, so `9000:9000` answers the internet even when ufw denies it. Bind `127.0.0.1:9000:9000` and put Caddy or nginx in front for TLS.

```sh
export KAMISATO_BUILD_API_KEY="$(openssl rand -hex 32)"
export AYATO_AUTH_USERNAME=ayato
export AYATO_AUTH_PASSWORD="$(openssl rand -hex 16)"
docker compose up -d
```

The repo `Dockerfile` ships the combined `kamisato` binary (which carries the `miko` subcommand) alongside standalone `ayato`/`ayaka`/`miko`/`lumine`. Build an image from it and point `image:` there; the Compose services run `kamisato ayato` / `kamisato miko`.

## GCP — Cloud Run skeleton

`terraform/` shows the resources and wiring, not a turnkey module. Check the attribute values against the provider docs before applying.

- `main.tf` — provider, a service account per service, the Secret Manager secret, and the IAM to read it.
- `cloudrun.tf` — the two services; ayato's service account gets `run.invoker` on miko and nobody else does. miko's ingress is `internal`, ayato's is public.
- `variables.tf` / `outputs.tf` — project, region, images in; service URLs out.

The boundary has two halves here too: network (miko's internal ingress) and identity (only ayato's service account may invoke miko). ayato reaches miko over direct VPC egress or a Serverless VPC Access connector — set one in `vpc_access`, since internal ingress admits nothing without a route into the VPC.

Cloud Run gives no Docker socket and no nested containers, so miko's `container` executor can't build there. A real GCP backend is Cloud Build or a GCE VM running the Compose stack; this Terraform is the network-and-identity skeleton around it.
