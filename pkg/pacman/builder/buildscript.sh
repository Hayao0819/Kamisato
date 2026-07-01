set -e
# Extra repositories (e.g. ayato) are appended to pacman.conf before the first
# sync so their databases are available to dependency resolution below.
__EXTRA_REPOS__
# makepkg refuses to run as root (exit 10); build as an unprivileged builduser.
# builduser keeps passwordless sudo because `makepkg --syncdeps` calls
# `sudo pacman -S` to install makedepends.
pacman -Sy --noconfirm --needed base-devel sudo
useradd -m -G wheel builduser 2>/dev/null || true
echo '%wheel ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/builduser
chmod 0440 /etc/sudoers.d/builduser

# Override only CARCH/CHOST by sourcing the base config first, so PKGEXT,
# compression, CFLAGS, MAKEFLAGS and PACKAGER defaults survive.
cat > /build/makepkg.override.conf <<'EOF'
source /etc/makepkg.conf
CARCH="__ARCH__"
CHOST="__ARCH__-pc-linux-gnu"
EOF

cp -r /build/src /build/work
chown -R builduser:builduser /build/work /build/makepkg.override.conf /build/out
__INSTALL__
sudo -u builduser env PKGDEST=/build/out sh -c 'cd /build/work && makepkg --config /build/makepkg.override.conf --syncdeps --noconfirm --clean'
