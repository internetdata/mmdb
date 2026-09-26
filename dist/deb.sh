#!/bin/sh

# Installs mmdb on Debian or Ubuntu, once.
#
# For updates, use the apt repository instead - see the README.

set -e

VSN="${VSN:-1.0.0}"
case "$(uname -m)" in
    x86_64)  ARCH=amd64 ;;
    i386|i686) ARCH=386 ;;
    aarch64) ARCH=arm64 ;;
    armv7l)  ARCH=arm ;;
    *)       echo "unsupported architecture: $(uname -m)" >&2 ; exit 1 ;;
esac

DEB="mmdb_${VSN}_linux_${ARCH}.deb"
URL="https://github.com/internetdata/mmdb/releases/download/v${VSN}/${DEB}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "downloading ${DEB}..."
curl -fLs "$URL" -o "${TMP}/${DEB}"

if [ "$(id -u)" -eq 0 ] ; then
    dpkg -i "${TMP}/${DEB}"
else
    sudo dpkg -i "${TMP}/${DEB}"
fi

echo
echo "installed. run 'mmdb --help'."
