#!/bin/sh
set -e -u
cd "$(dirname "$0")/miko" || exit 1
go run . -- "$@"
