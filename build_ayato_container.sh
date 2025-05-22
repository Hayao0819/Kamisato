#!/bin/sh

cd "$(dirname "$0")" || exit 1
docker build -f ./ayato/Dockerfile -t ayato-app "$@" .
