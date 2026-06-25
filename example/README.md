# Example

A runnable demo: ayato + miko + lumine in Docker, driven by the ayaka CLI from
the host. See the component READMEs for what each piece does.

`config/` holds the ayato and miko config (shared API key `example-build-key`).
`ayaka/myrepo/` holds example PKGBUILD sources; `dummypkg` is used below.

ayato's `auth.trusted_proxies` lists the addresses allowed to set `X-Forwarded-*`
so that `c.ClientIP()` (the per-IP rate-limit key) reflects the real client
rather than a spoofed header. It must cover lumine's source address; here it is
the compose bridge subnet (`172.16.0.0/12`). Set it to lumine's actual CIDR in a
real deployment. Leaving it empty now means trust NONE — `X-Forwarded-For` is
ignored and `ClientIP()` falls back to the direct peer — so the rate-limit key is
only meaningful when lumine is the sole hop setting that header.

## Run

From the repository root:

```sh
docker compose -f example/compose.yml up -d --build

ayaka server add http://localhost:8080 ayato password
cd example/ayaka
ayaka miko build myrepo dummypkg --server http://localhost:8080
ayaka miko logs <job-id> --server http://localhost:8080
```

ayato then serves the built package (lumine at <http://localhost:3000>):

```sh
curl http://localhost:8080/api/unstable/myrepo/x86_64/package
```

Tear down with `docker compose -f example/compose.yml down -v`.
