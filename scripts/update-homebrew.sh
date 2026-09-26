#!/bin/bash

# Updates the Homebrew formula in internetdata/homebrew-tap for a release.
#
# Our own tap rather than homebrew-core, which requires a notability the project
# does not have yet. `brew tap internetdata/tap` then `brew install mmdb` is the
# install, and `brew upgrade` keeps it current.

set -euo pipefail

cd "$(dirname "$0")/.."

VSN="${1:?usage: update-homebrew.sh <version>}"
TAP_DEPLOY_KEY="${TAP_DEPLOY_KEY:?a deploy key with push access to the tap is required}"
REPO="internetdata/homebrew-tap"
BASE="https://github.com/internetdata/mmdb/releases/download/v${VSN}"

# Hashes are taken from the release's own artifacts rather than from the local
# build, so the formula describes what people will actually download.
function sha_of() {
    curl -fsSL "${BASE}/$1" | sha256sum | cut -d' ' -f1
}

echo "==> Hashing release artifacts..."
DARWIN_ARM="$(sha_of "mmdb_${VSN}_darwin_arm64.tar.gz")"
DARWIN_AMD="$(sha_of "mmdb_${VSN}_darwin_amd64.tar.gz")"
LINUX_ARM="$(sha_of "mmdb_${VSN}_linux_arm64.tar.gz")"
LINUX_AMD="$(sha_of "mmdb_${VSN}_linux_amd64.tar.gz")"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
# A deploy key, because GitHub has no API to mint a token: it reaches the tap
# and nothing else in the org, which a personal access token would not.
printf '%s\n' "$TAP_DEPLOY_KEY" > "$TMP/key"
chmod 600 "$TMP/key"
export GIT_SSH_COMMAND="ssh -i $TMP/key -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"

git clone -q "git@github.com:${REPO}.git" "$TMP/tap"

mkdir -p "$TMP/tap/Formula"
sed -e "s|@VSN@|${VSN}|g" \
    -e "s|@DARWIN_ARM@|${DARWIN_ARM}|g" \
    -e "s|@DARWIN_AMD@|${DARWIN_AMD}|g" \
    -e "s|@LINUX_ARM@|${LINUX_ARM}|g" \
    -e "s|@LINUX_AMD@|${LINUX_AMD}|g" \
    homebrew/mmdb.rb.tmpl > "$TMP/tap/Formula/mmdb.rb"

cd "$TMP/tap"
# `git diff` reads tracked files only, so a tap with no formula yet reports no
# change and this would exit claiming success having pushed nothing.
git add -A Formula
if git diff --cached --quiet ; then
    echo "==> Formula already at ${VSN}; nothing to do."
    exit 0
fi
git -c user.name="internetdata-bot" -c user.email="support@internetdata.io" \
    commit -qm "mmdb ${VSN}"
git push -q origin HEAD
echo "==> ${REPO} updated to ${VSN}"
