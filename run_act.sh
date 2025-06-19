#!/usr/bin/env bash

set -e -u -o pipefail
current_dir=$(cd "$(dirname "$0")" && pwd)

vars_file="${current_dir}/.github/act/.vars"
secrets_file="${current_dir}/.github/act/.secrets"

act_args=(
    "--var-file" "${vars_file}"
)

if [[ -e "${secrets_file}" ]]; then
    act_args+=("--secret-file" "${secrets_file}")
fi

exec act "${act_args[@]}" "$@"
