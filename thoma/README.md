# Thoma | トーマ

A drop-in `makepkg` that builds on miko (via ayato) instead of locally, so yay
works unchanged on low-powered machines. Only the compile is sent to the build
server; the rest (source download, `--packagelist`, install) stays local.

## Use

```sh
ayaka server login https://ayato.example.com   # once, for the token
yay --makepkg /usr/bin/thoma -S <pkg>          # or set makepkgbin in yay.conf
```

## Environment

| Variable | Meaning | Default |
|---|---|---|
| `THOMA_REPO` | ayato repo to build into (required) | — |
| `THOMA_SERVER` | ayato server URL | default server |
| `THOMA_ARCH` | build architecture | `uname -m` |
| `THOMA_MAKEPKG` | real makepkg | `/usr/bin/makepkg` |
| `THOMA_GPGKEY` | signing key | none |
| `THOMA_TIMEOUT` | timeout, minutes | miko default |
