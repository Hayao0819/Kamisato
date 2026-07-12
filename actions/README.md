# Kamisato composite actions

Use from another repo as `Hayao0819/Kamisato/actions/<name>@<ref>`.

- `install` — install selected Kamisato CLIs (ayaka/ayato/miko/lumine) and add them to `PATH`.
- `upload` — publish package files to ayato (`ayaka repo add`); needs `install` first.
- `prune` — delete packages from ayato that no longer exist in the checked-out source repo (`ayaka repo remove --diff`); needs `install` first.
- `build-lumine` — build the lumine web UI to a static directory with `env.json`/CSP injected, ready for any static host (the host-specific upload is the caller's step).

```yaml
- uses: actions/setup-go@v5
  with: { go-version: "1.x" }
- uses: Hayao0819/Kamisato/actions/install@<ref>
  with: { ayaka: "true" }          # method=source, version=main by default
- uses: Hayao0819/Kamisato/actions/upload@<ref>
  with:
    server: https://repo.example.com
    repo: alterlinux
    token: ${{ secrets.AYATO_TOKEN }}
    files: out/alterlinux/**/*.pkg.tar.*
```

```yaml
# build the static site, then upload it wherever it is hosted
- uses: Hayao0819/Kamisato/actions/build-lumine@<ref>
  id: lumine
  with:
    ayato_url: https://repo.example.com
    auth_mode: bearer
- uses: cloudflare/wrangler-action@v3
  with:
    apiToken: ${{ secrets.CLOUDFLARE_API_TOKEN }}
    accountId: ${{ vars.CF_ACCOUNT_ID }}
    command: pages deploy ${{ steps.lumine.outputs.dir }} --project-name=alterlinux --branch=main
```

See each `action.yml` for the full input list.

`install` defaults to `method: source`, which builds from a checkout; it needs
Go, plus pnpm for lumine. The `v0.0.1`/`v0.0.2` release binaries predate the
current CLI, so `method: release` (prebuilt ayaka/ayato/lumine, no miko) is only
for newer tags.
