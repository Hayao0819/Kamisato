#!/bin/sh
set -e -u
cd "$(dirname "$0")/ayato" || exit 1
go run . -- "$@"
