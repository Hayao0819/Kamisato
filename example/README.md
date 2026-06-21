# Example

A runnable demo: ayato + miko + lumine in Docker, driven by the ayaka CLI from
the host. See the component READMEs for what each piece does.

`config/` holds the ayato and miko config (shared API key `example-build-key`).
`ayaka/myrepo/` holds example PKGBUILD sources; `dummypkg` is used below.

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
