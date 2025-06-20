#!/bin/sh
set -e -u
cd "$(dirname "$0")/ayaka" || exit 1
go run . "$@"
