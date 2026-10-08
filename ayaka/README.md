# Ayaka | 綾華

Ayaka is the command-line client for a Kamisato repository. It manages local
PKGBUILD sources and either builds them locally or submits them to
miko for a server-side build, then inspects and publishes the results on ayato.

## Supported environment

Local source-repository builds use the configured chroot, container, or bwrap
backend. Chroot builds require Arch Linux and devtools; direct package builds
use the container backend. Remote builds (`ayaka miko`) work from any host.

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

- `build` build locally from configured source repositories or explicit packages
- `plan` resolve explicit packages and their build dependencies without building
- `miko` submit and inspect server-side builds (`build`, `jobs`, `status`, `logs`, `cancel`, `stats`)
- `repo` publish packages to ayato
- `server` manage ayato endpoints
- `src` manage local sources (`list`, `status`, `srcinfo`, `bump`, `nvbump`, `pull`, `aur`, `submodules`)
- `ci` emit CI plans, build matrices, and upstream version checks
- `key` manage private signing keys and subkeys
- `keyring` build, publish, and bootstrap public keyring packages
- `hook` install the package-upload hook
