# Ayaka | 綾華

Ayaka is the command-line client for a Kamisato repository. It manages local
PKGBUILD sources and either builds them locally (clean chroot) or submits them to
miko for a server-side build, then inspects and publishes the results on ayato.

## Supported environment

Local chroot builds run only on Arch Linux (devtools). Remote builds (`ayaka miko`)
work from any host.

## Configuration

`.ayakarc.json` holds host-trusted builder settings and source repositories.
Keep executable selection here; a repository-owned `repo.json` cannot select
host commands:

```json
{
    "builder": {
        "backend": "chroot",
        "devtools": {
            "archbuild": "extra-x86_64-build"
        }
    },
    "repos": [
        { "dir": "./myrepo", "destdir": "./out" }
    ]
}
```

Each source repository has a `repo.json` next to its packages:

```json
{
    "name": "myrepo",
    "maintainer": "hayao <shun819.mail at gmail.com>",
    "url": "",
    "build": {
        "timeout": "30m"
    }
}
```

See [../example/ayaka](../example/ayaka) for a working sample.

## Subcommands

- `build` build locally, or `--remote` to build on miko
- `miko` submit and inspect server-side builds (`build`, `jobs`, `status`, `logs`, `cancel`, `stats`)
- `repo` publish packages to ayato
- `server` manage ayato endpoints
- `list` / `status` show source packages and their build state
- `aur` manage PKGBUILDs taken from the AUR
