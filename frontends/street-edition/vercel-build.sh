#!/bin/sh
# Vercel's build step for Street Edition (#411): the playable site is
# dist/, which build.py makes from the whole checkout (the engine
# compiled to WebAssembly, the reference client's helpers, the content's
# words). Vercel's build image has no Go, so this fetches the version
# go.mod names, and build.py needs a Python with tomllib (3.11+). Run
# from this directory, as Vercel does with the root directory set here.
set -eu
cd "$(dirname "$0")"
root=../..

version=$(awk '$1 == "go" { print $2; exit }' "$root/go.mod")
if command -v go >/dev/null 2>&1 && [ "$(go env GOVERSION)" = "go$version" ]; then
  go=go
else
  arch=$(uname -m)
  case "$arch" in
    x86_64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
  esac
  dir=${TMPDIR:-/tmp}/street-edition-go$version
  if [ ! -x "$dir/go/bin/go" ]; then
    mkdir -p "$dir"
    echo "fetching go$version for linux-$arch"
    curl -fsSL "https://go.dev/dl/go$version.linux-$arch.tar.gz" | tar -xz -C "$dir"
  fi
  go=$dir/go/bin/go
fi

python=
for p in python3.13 python3.12 python3.11 python3; do
  if command -v "$p" >/dev/null 2>&1 && "$p" -c 'import tomllib' 2>/dev/null; then
    python=$p
    break
  fi
done
if [ -z "$python" ]; then
  echo "no Python 3.11+ (build.py needs tomllib)" >&2
  exit 1
fi

GOTOOLCHAIN=local KINGPIN_GO=$go "$python" build.py
