#!/usr/bin/env bash
# Minimal native Arch Linux demo — no Docker, no setup.
#
# Builds the kamisato binary, runs ayato (repo server) and lumine (web UI),
# builds a package with makepkg, publishes it, and serves it as a real pacman
# repository. Everything lives in a throwaway temp dir and is cleaned up on exit.
#
# Needs only an Arch box with: go, base-devel (makepkg/fakeroot), pacman
# (repo-add), curl. For the richer, configurable stack use compose.yml instead.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/.." && pwd)"

for c in go makepkg fakeroot repo-add curl; do
	command -v "$c" >/dev/null 2>&1 ||
		{ echo "missing '$c' — install with: sudo pacman -S --needed go base-devel pacman curl"; exit 1; }
done

work="$(mktemp -d)"
bin="$work/bin"
data="$work/data"
mkdir -p "$bin" "$data"

ayato_pid=""
lumine_pid=""
cleanup() {
	[ -n "$lumine_pid" ] && kill "$lumine_pid" 2>/dev/null || true
	[ -n "$ayato_pid" ] && kill "$ayato_pid" 2>/dev/null || true
	rm -rf "$work"
}
trap cleanup EXIT INT TERM

echo "==> building kamisato"
(cd "$root" && go build -o "$bin/kamisato" .)

key="local-demo-key"
cat >"$work/ayato.json" <<EOF
{
  "port": 8080,
  "auth": { "ci": { "api_keys": [ { "name": "demo", "key": "$key", "publish_repos": ["myrepo"] } ] } },
  "store": { "db_type": "badgerdb", "badgerdb": "$data/db", "storage_type": "localfs", "local_repo_dir": "$data/repo" },
  "repos": [ { "name": "myrepo", "arches": ["x86_64"] } ]
}
EOF

echo "==> starting ayato on :8080"
"$bin/kamisato" ayato -c "$work/ayato.json" >"$work/ayato.log" 2>&1 &
ayato_pid=$!
curl -fsS --retry 30 --retry-delay 1 --retry-connrefused -o /dev/null \
	http://127.0.0.1:8080/api/unstable/hello

echo "==> building dummypkg with makepkg"
build="$work/build"
mkdir -p "$build"
cp "$here/ayaka/myrepo/dummypkg/PKGBUILD" "$build/"
(cd "$build" && PKGDEST="$build" makepkg -f --nodeps --skipinteg >/dev/null)
shopt -s nullglob
pkgs=("$build"/dummypkg-*.pkg.tar.*)
pkg="${pkgs[0]}"

echo "==> publishing $(basename "$pkg")"
curl -fsS -o /dev/null -w "    upload: HTTP %{http_code}\n" \
	-X PUT -H "X-API-Key: $key" -F "package=@$pkg" \
	http://127.0.0.1:8080/api/unstable/myrepo/package

echo "==> starting lumine (web UI) on :3000"
"$bin/kamisato" lumine --addr 127.0.0.1:3000 --ayato-url http://127.0.0.1:8080 \
	>"$work/lumine.log" 2>&1 &
lumine_pid=$!

cat <<EOF

  kamisato is running. Press Ctrl-C to stop and clean up.

    Web UI   http://127.0.0.1:3000
    API      http://127.0.0.1:8080/api/unstable/repos
    Package  http://127.0.0.1:8080/api/unstable/myrepo/x86_64/package

  Install it with pacman by adding to /etc/pacman.conf:

    [myrepo]
    SigLevel = Optional TrustAll
    Server = http://127.0.0.1:8080/repo/myrepo/\$arch

  then run: sudo pacman -Sy dummypkg

EOF

wait
