#!/bin/bash
# bwrap phase 1 (root-in-userns): install the PKGBUILD's dependencies into the
# shared overlay so the non-root phase 2 can run makepkg without --syncdeps.
# __EXTRA_REPOS__ and __INSTALL__ are substituted by bwrap.go before this runs.
set -euo pipefail

# pacman 7's download sandbox (Landlock/seccomp + the DownloadUser switch) fails
# inside a single-uid user namespace; disable it for this environment.
sed -i '/^\[options\]/a DisableSandbox' /etc/pacman.conf

# Extra repositories (e.g. ayato) so already-published dependencies resolve below.
__EXTRA_REPOS__

pacman -Sy --noconfirm --needed base-devel

# A passwd/group entry so the non-root build phase (uid 1000) resolves to a name.
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
	# depends/makedepends/checkdepends come from the sourced PKGBUILD.
	# shellcheck disable=SC2154
	printf '%s ' "${depends[@]}" "${makedepends[@]}" "${checkdepends[@]}"
)
if [[ -n "${deps// /}" ]]; then
	# Intentional word splitting: pass each dependency as a separate argument.
	# shellcheck disable=SC2086
	pacman -S --asdeps --needed --noconfirm $deps
fi
__INSTALL__

# The build dir must be writable by the non-root build phase.
chmod -R a+rwX /build 2>/dev/null || true
