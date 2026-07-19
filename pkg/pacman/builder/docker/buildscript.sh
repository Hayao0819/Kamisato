#!/bin/bash
set -euo pipefail

: "${TARGET_CARCH:?TARGET_CARCH must be set by the container backend}"
: "${TARGET_CHOST:?TARGET_CHOST must be set by the container backend}"

# Add repositories before the first sync so they participate in dependency resolution.
__EXTRA_REPOS__

# makepkg refuses to run as root (exit 10); build as an unprivileged builduser.
# builduser keeps passwordless sudo because `makepkg --syncdeps` runs
# `sudo pacman -S` to install makedepends.
pacman -Sy --noconfirm --needed base-devel sudo
useradd -m -G wheel builduser 2>/dev/null || true
printf '%s\n' '%wheel ALL=(ALL) NOPASSWD: ALL' >/etc/sudoers.d/builduser
chmod 0440 /etc/sudoers.d/builduser

# Override only CARCH/CHOST: copy the staged base config (which sources
# /etc/makepkg.conf, preserving PKGEXT, compression, CFLAGS, MAKEFLAGS and
# PACKAGER) and append the dynamic arch lines.
cp /build/staging/makepkg.override.conf /build/makepkg.override.conf
{
	printf 'CARCH="%s"\n' "$TARGET_CARCH"
	printf 'CHOST="%s"\n' "$TARGET_CHOST"
} >>/build/makepkg.override.conf

# src is mounted read-only; copy it to a writable work dir so the caller's tree
# is never mutated.
cp -r /build/src /build/work
chown -R builduser:builduser /build/work /build/makepkg.override.conf
# /build/out is an isolated per-build host staging directory. Keep its host
# ownership intact so the caller can move artifacts after the container exits,
# while allowing the unprivileged build user to create package files in it.
chmod 0777 /build/out
__INSTALL__
sudo -u builduser env PKGDEST=/build/out sh -c 'cd /build/work && makepkg --config /build/makepkg.override.conf --syncdeps --noconfirm --clean'
