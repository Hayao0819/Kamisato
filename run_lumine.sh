#!/bin/sh
set -e -u
cd "$(dirname "$0")/lumine/web" || exit 1
pnpm run dev -- "$@"
