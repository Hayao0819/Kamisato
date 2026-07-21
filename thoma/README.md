# Thoma | トーマ

A `makepkg` drop-in that sends the compile to a miko builder instead of running
it locally, so an underpowered machine can keep using yay. thoma offloads only
the build itself. Source download, `--packagelist`, `.SRCINFO`, and the install
all fall through to the real makepkg on the local box.

## Modes

`THOMA_MODE` picks where the build runs and where the finished package comes
from.

`ayato` (the default) submits through an ayato server and installs the
published, host-signed package from ayato's repo. The build lands in the shared
repo like any other ayato build.

`direct` submits straight to a miko builder. Set `THOMA_SERVER` to the builder's
URL and `THOMA_API_KEY` to a miko key. thoma pulls the unsigned artifact back
from the job and installs that; nothing is published. Use it when you want to
build on a fast box on the LAN and install locally without touching the shared
repo.

## Use

```sh
ayaka server login https://ayato.example.com   # once, to get the token
yay --makepkg /usr/bin/thoma -S <pkg>          # or set makepkgbin in yay.conf
```

## Configuration

Settings come from `THOMA_*` environment variables or a
`.thomarc.{json,toml,yaml}` with snake_case keys (`api_key`), resolved through
koanf.

| Variable | Meaning | Default |
|---|---|---|
| `THOMA_REPO` | repo to build into (required) | — |
| `THOMA_MODE` | `ayato` or `direct` | `ayato` |
| `THOMA_SERVER` | ayato server URL, or the miko URL in direct mode | default server |
| `THOMA_API_KEY` | miko api key (required in direct mode) | — |
| `THOMA_ARCH` | build architecture | makepkg.conf `CARCH`, else `uname -m` |
| `THOMA_MAKEPKG` | real makepkg | `/usr/bin/makepkg` |
| `THOMA_TIMEOUT` | timeout, minutes | miko default |

There is no client-side signing (the old `THOMA_GPGKEY`). The build host signs
the package, so thoma never reads a signing key.
