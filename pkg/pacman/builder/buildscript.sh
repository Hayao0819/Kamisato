#!/bin/bash
# Container backend entrypoint. __EXTRA_REPOS__ and __INSTALL__ are substituted
# by container.go before this runs; TARGET_CARCH is passed via the environment.
set -euo pipefail

: "${TARGET_CARCH:?TARGET_CARCH must be set by the container backend}"

# Extra repositories (e.g. ayato) are appended to pacman.conf before the first
# sync so their databases feed dependency resolution below.
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
	printf 'CHOST="%s-pc-linux-gnu"\n' "$TARGET_CARCH"
} >>/build/makepkg.override.conf

# src is mounted read-only; copy it to a writable work dir so the caller's tree
# is never mutated.
cp -r /build/src /build/work
chown -R builduser:builduser /build/work /build/makepkg.override.conf /build/out
__INSTALL__
sudo -u builduser env PKGDEST=/build/out sh -c 'cd /build/work && makepkg --config /build/makepkg.override.conf --syncdeps --noconfirm --clean'
