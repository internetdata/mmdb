#!/bin/bash

# Refuses a release whose version strings do not all agree with the tag.
#
#   ./scripts/check-versions.sh 0.2.2
#
# `main.go` is the version of record. Everything else checked here is a COPY of
# it that a release silently invalidates: the installer scripts hand out a
# download URL built from their own default, so a stale one keeps serving the
# PREVIOUS release forever, and it works - which is why nobody notices.
#
# Deliberately NOT checked. Each is a template a build step substitutes, and a
# gate that flags a template is a gate someone switches off:
#   dist/DEBIAN/control            Version: 0.0.0   <- scripts/build-archive-all.sh
#   chocolatey-packages/*.nuspec   <version>0.0.0   <- scripts/update-chocolatey.ps1
#   homebrew/mmdb.rb.tmpl          @VSN@            <- scripts/update-homebrew.sh
#   Dockerfile                     ARG VERSION=dev  <- --build-arg, release.yml

set -uo pipefail

cd "$(dirname "$0")/.."

WANT="${1:?usage: check-versions.sh <version>, e.g. 0.2.2}"
WANT="${WANT#v}"

# Every file that MUST state the version. Empty output is a failure rather than
# a pass: it means the pattern stopped matching, and a gate that silently checks
# nothing is worse than no gate.
REQUIRED="main.go dist/deb.sh dist/macos.sh dist/windows.ps1"

# Prints the version(s) a file declares, one per line, anchored on that file's
# own syntax. The anchoring is the whole difficulty: these files are full of
# numbers that are not the release - a Go toolchain floor, an example IP
# address, a shields.io badge - and a pattern loose enough to catch a stale
# version catches those too.
function declared_in() {
    local f="$1"
    case "$f" in
        *.go)   grep -oE 'var version = "[0-9][0-9.]*"' "$f" ;;
        *.sh)   grep -oE 'VSN:-[0-9][0-9.]+' "$f" ;;
        *.ps1)  grep -oE 'else \{ "[0-9][0-9.]+"' "$f" ;;
        # A README names the release only inside a release-artifact filename or
        # a download URL, never in prose.
        *.md)   grep -oE "mmdb_[0-9][0-9.]*_[a-z0-9]+_[a-z0-9]+\.(tar\.gz|zip)|releases/download/v[0-9][0-9.]*/" "$f" ;;
    esac | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?' | sort -u
}

rc=0

for f in $REQUIRED ; do
    if [ ! -f "$f" ] ; then
        echo "  FAIL ${f}: missing - this gate names a file that no longer exists" >&2
        rc=1
        continue
    fi
    found="$(declared_in "$f")"
    if [ -z "$found" ] ; then
        echo "  FAIL ${f}: states no version - the pattern matching it has drifted" >&2
        rc=1
        continue
    fi
    while read -r v ; do
        [ -n "$v" ] || continue
        if [ "$v" != "$WANT" ] ; then
            echo "  FAIL ${f}: says ${v}, the release is ${WANT}" >&2
            rc=1
        fi
    done <<< "$found"
done

# The README may name no version at all, and that is the better state - a
# `releases/latest/download/` URL never goes stale. But a version it DOES name
# is the first thing a customer copies, so it has to be this one.
for v in $(declared_in README.md) ; do
    if [ "$v" != "$WANT" ] ; then
        echo "  FAIL README.md: pins ${v}, the release is ${WANT}" >&2
        echo "       a pinned version inside a /releases/latest/download/ URL 404s" >&2
        rc=1
    fi
done

if [ "$rc" -ne 0 ] ; then
    echo "==> FAILED - bump every version string, then re-tag" >&2
    exit 1
fi
echo "==> versions agree on ${WANT}"
