set -e
# Phase 1 runs as root-in-userns. pacman 7's download sandbox (Landlock/seccomp,
# and the DownloadUser switch) fails inside a single-uid user namespace, so turn
# it off for the build environment.
sed -i '/^\[options\]/a DisableSandbox' /etc/pacman.conf

# Extra repositories (e.g. ayato) so already-published dependencies resolve below.
__EXTRA_REPOS__

pacman -Sy --noconfirm --needed base-devel

# A passwd/group entry so the non-root build phase (uid 1000) resolves to a name.
grep -q '^builder:' /etc/passwd || echo 'builder:x:1000:1000::/build:/bin/bash' >> /etc/passwd
grep -q '^builder:' /etc/group || echo 'builder:x:1000:' >> /etc/group

cd /build
# Install the PKGBUILD's declared dependencies from the configured repos. This is
# the dependency-install step that the non-root makepkg phase cannot do itself.
deps=$( . ./PKGBUILD 2>/dev/null; printf '%s ' "${depends[@]}" "${makedepends[@]}" "${checkdepends[@]}" )
if [ -n "${deps// /}" ]; then
	pacman -S --asdeps --needed --noconfirm $deps
fi
__INSTALL__

# The build dir must be writable by the non-root build phase.
chmod -R a+rwX /build 2>/dev/null || true
