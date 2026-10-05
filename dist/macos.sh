#!/bin/sh

# Installs mmdb on macOS.
#
#   curl -Ls https://github.com/internetdata/mmdb/releases/latest/download/macos.sh | sh

set -e

VSN="${VSN:-1.2.0}"

# Go 1.27 builds the release and needs macOS 13 Ventura.
os="$(sw_vers -productVersion 2>/dev/null || true)"
if [ "${os%%.*}" -lt 13 ] 2>/dev/null ; then
    echo "mmdb needs macOS 13 Ventura or later; this is macOS ${os}." >&2
    exit 1
fi

# Under Rosetta, uname says x86_64 on Apple Silicon, and an amd64 binary stops
# running once Rosetta is gone. sysctl.proc_translated is 1 only in a translated process.
MACHINE="$(uname -m)"
if [ "$MACHINE" = x86_64 ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null)" = 1 ] ; then
    MACHINE=arm64
fi
case "$MACHINE" in
    arm64)  ARCH=arm64 ;;
    x86_64) ARCH=amd64 ;;
    *)      echo "unsupported architecture: ${MACHINE}" >&2 ; exit 1 ;;
esac

TARBALL="mmdb_${VSN}_darwin_${ARCH}.tar.gz"
URL="https://github.com/internetdata/mmdb/releases/download/v${VSN}/${TARBALL}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "downloading ${TARBALL}..."
curl -fLs "$URL" -o "${TMP}/${TARBALL}"
tar -xzf "${TMP}/${TARBALL}" -C "$TMP"

# /usr/local/bin needs root on most machines and does not on some; only ask
# when it is actually needed.
if [ -w /usr/local/bin ] ; then
    mv "${TMP}/mmdb" /usr/local/bin/mmdb
else
    sudo mv "${TMP}/mmdb" /usr/local/bin/mmdb
fi

echo
echo "installed. run 'mmdb --help', or 'mmdb completion install'"
echo "for shell auto-completion."
echo
echo "macOS may refuse to run it the first time: the binary is not notarized."
echo "Allow it under System Settings > Privacy & Security."
