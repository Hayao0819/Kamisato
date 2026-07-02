# Thoma | トーマ

A drop-in `makepkg` that builds on miko instead of locally, so yay works
unchanged on low-powered machines. Only the compile is sent to the build server;
the rest (source download, `--packagelist`, install) stays local.

## Modes

`THOMA_MODE` selects where the build runs and where the package comes from:

- `ayato` (default): submit through an ayato server and install the published,
  host-signed package from ayato's repo. The build ends up in the shared repo.
- `direct`: submit straight to a miko builder (set `THOMA_SERVER` to its URL and
  `THOMA_API_KEY` to a miko key) and install the unsigned artifact pulled from
  the job. Nothing is published to the shared repo — this is the LAN-dev win:
  build on a beefy box, install locally, without touching the published repo.

## Use

```sh
ayaka server login https://ayato.example.com   # once, for the token
yay --makepkg /usr/bin/thoma -S <pkg>          # or set makepkgbin in yay.conf
```

## Configuration

Settings come from `THOMA_*` environment variables or a
`.thomarc.{json,toml,yaml}` (snake_case keys, e.g. `api_key`), in the usual
koanf precedence.

| Variable | Meaning | Default |
|---|---|---|
| `THOMA_REPO` | repo to build into (required) | — |
| `THOMA_MODE` | `ayato` or `direct` | `ayato` |
| `THOMA_SERVER` | ayato server URL, or the miko URL in direct mode | default server |
| `THOMA_API_KEY` | miko api key (required in direct mode) | — |
| `THOMA_ARCH` | build architecture | `uname -m` |
| `THOMA_MAKEPKG` | real makepkg | `/usr/bin/makepkg` |
| `THOMA_TIMEOUT` | timeout, minutes | miko default |

Client-side signing (the old `THOMA_GPGKEY`) is not implemented; packages are
signed by the build host, so no signing key is read here.
