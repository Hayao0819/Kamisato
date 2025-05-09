#!/bin/sh
set -e -u
cd "$(dirname "$0")/lumine" || exit 1
pnpm run dev -- "$@"
