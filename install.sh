#!/bin/sh

set -e -u

current_dir=$(cd "$(dirname "$0")" && pwd)
bin_dir="/bin"

build_ayaka=true
build_ayato=true
build_lumine_go=true
build_lumine_web=false

cd "$current_dir" || exit 1

print_usage() {
    echo "Usage: $0 [--help|-h] [--bin <path>]"
    echo "Install script for the project."
}

check_requirements_go() {
    if ! command -v go >/dev/null 2>&1; then
        echo "Error: Go is not installed. Please install Go to proceed."
        return 1
    fi

    if ! go version >/dev/null 2>&1; then
        echo "Error: Go is not properly configured. Please check your Go installation."
        return 1
    fi

    return 0
}

check_requirements_pnpm() {
    if ! command -v pnpm >/dev/null 2>&1; then
        echo "Error: pnpm is not installed. Please install pnpm to proceed."
        return 1
    fi

    if ! pnpm --version >/dev/null 2>&1; then
        echo "Error: pnpm is not properly configured. Please check your pnpm installation."
        return 1
    fi

    return 0
}

check_requirements() {
    if [ "$build_ayaka" = true ] || [ "$build_ayato" = true ] || [ "$build_lumine_go" = true ]; then
        if ! check_requirements_go; then
            return 1
        fi
    fi

    if [ "$build_lumine_web" = true ]; then
        check_requirements_pnpm || return 1
    fi

    return 0
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
        -b | --bin)
            if [ $# -lt 2 ]; then
                echo "Error: --bin requires an argument."
                return 1
            fi
            if [ ! -d "$2" ]; then
                echo "Error: Directory '$2' does not exist."
                return 1
            fi
            bin_dir="$(cd "$2" && pwd)"
            shift 2
            ;;
        --no-lumine-web)
            build_lumine_web=false
            shift
            ;;
        --no-lumine-go)
            build_lumine_go=false
            shift
            ;;
        --no-ayaka)
            build_ayaka=false
            shift
            ;;
        --no-ayato)
            build_ayato=false
            shift
            ;;
        --help | -h)
            print_usage
            return 0
            ;;
        *)
            echo "Unknown option: $1"
            print_usage
            return 1
            ;;
        esac
    done
}

build_go() {
    (
        cd "$1" || exit 1
        go build -o "$bin_dir/$(basename "$1")" .
    )
}

build_ayaka() {
    build_go "$current_dir/ayaka"
}

build_ayato() {
    build_go "$current_dir/ayato"
}

build_nextjs() {
    (
        cd "$1" || exit 1
        pnpm install
        pnpm run build
    )
}

build_lumine_web() {
    build_nextjs "$current_dir/lumine/web"
}

build_lumine_go() {
    if ! [ -e "$current_dir/lumine/embed/out" ]; then
        echo "Error: Lumine embed directory does not exist." >&2
        return 1
    fi
    build_go "$current_dir/lumine"
}

main() {
    if ! parse_args "$@"; then
        exit 1
    fi

    check_requirements

    if [ $build_ayaka = true ]; then
        build_ayaka
    fi

    if [ $build_ayato = true ]; then
        build_ayato
    fi

    if [ "$build_lumine_web" = true ]; then
        build_lumine_web
    fi

    if [ "$build_lumine_go" = true ]; then
        build_lumine_go
    fi
}

main "$@"
