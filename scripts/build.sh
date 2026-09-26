#!/bin/bash

# Builds the binary for this machine, into build/.

set -euo pipefail

cd "$(dirname "$0")/.."

VSN="$(grep -m1 'var version = ' main.go | cut -d'"' -f2)"

mkdir -p build
go build -ldflags "-s -w -X main.version=${VSN}" -o build/mmdb .
echo "build/mmdb (${VSN})"
