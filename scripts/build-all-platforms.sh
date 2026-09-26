#!/bin/bash

# Cross-compiles for every platform a release ships, into build/.
#
# Pure Go with no cgo, so every target builds from one machine with no
# toolchain beyond the one already here.

set -euo pipefail

cd "$(dirname "$0")/.."

VSN="${1:-$(grep -m1 'var version = ' main.go | cut -d'"' -f2)}"
LINUX_ONLY="${2:-false}"

TARGETS=(
    darwin_amd64 darwin_arm64
    dragonfly_amd64
    freebsd_386 freebsd_amd64 freebsd_arm freebsd_arm64
    linux_386 linux_amd64 linux_arm linux_arm64
    netbsd_386 netbsd_amd64 netbsd_arm netbsd_arm64
    openbsd_386 openbsd_amd64 openbsd_arm openbsd_arm64
    solaris_amd64
    # No windows_arm: Go dropped 32-bit Windows on ARM, and asking for it
    # fails the whole run with "unsupported GOOS/GOARCH pair".
    windows_386 windows_amd64 windows_arm64
)

rm -rf build && mkdir -p build

for target in "${TARGETS[@]}" ; do
    os="${target%_*}"
    arch="${target#*_}"

    if [ "$LINUX_ONLY" = true ] && [ "$os" != linux ] ; then
        continue
    fi

    out="build/mmdb_${VSN}_${os}_${arch}"
    if [ "$os" = windows ] ; then
        out+=".exe"
    fi

    echo "building ${out}"
    # Trimpath so the binary does not carry this machine's directory layout,
    # and -s -w to drop the symbol table a CLI has no use for.
    GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build \
        -trimpath -ldflags "-s -w -X main.version=${VSN}" -o "$out" .
done

echo "built $(ls build | wc -l) binaries for ${VSN}"
