#!/bin/bash
set -euo pipefail

# pacman 7's download sandbox (Landlock/seccomp + the DownloadUser switch) fails
# inside a single-uid user namespace; disable it for this environment.
sed -i '/^\[options\]/a DisableSandbox' /etc/pacman.conf

__EXTRA_REPOS__

pacman -Sy --noconfirm --needed base-devel

grep -q '^builder:' /etc/passwd || echo 'builder:x:1000:1000::/build:/bin/bash' >>/etc/passwd
grep -q '^builder:' /etc/group || echo 'builder:x:1000:' >>/etc/group

# Install the PKGBUILD's declared dependencies — the step the non-root makepkg
# phase cannot perform itself. Sourcing runs with nounset off because a PKGBUILD
# may leave the dependency arrays undefined.
cd /build
deps=$(
	set +u
	# shellcheck source=/dev/null
	. ./PKGBUILD 2>/dev/null
	# shellcheck disable=SC2154
	printf '%s ' "${depends[@]}" "${makedepends[@]}" "${checkdepends[@]}"
)
if [[ -n "${deps// /}" ]]; then
	# Intentional word splitting: pass each dependency as a separate argument.
	# shellcheck disable=SC2086
	pacman -S --asdeps --needed --noconfirm $deps
fi
__INSTALL__

chmod -R a+rwX /build 2>/dev/null || true
