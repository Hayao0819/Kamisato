# Kamisato composite actions

Use from another repo as `Hayao0819/Kamisato/actions/<name>@<ref>`.

- `install` — install selected Kamisato CLIs (ayaka/ayato/miko/lumine) and add them to `PATH`.
- `upload` — publish package files to ayato (`ayaka repo add`); needs `install` first.

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

See each `action.yml` for the full input list.

`install` defaults to `method: source`, which builds from a checkout; it needs
Go, plus pnpm for lumine. The `v0.0.1`/`v0.0.2` release binaries predate the
current CLI, so `method: release` (prebuilt ayaka/ayato/lumine, no miko) is only
for newer tags.
