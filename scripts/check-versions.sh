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
# It also refuses a version with no CHANGELOG.md section of its own, which
# release.yml publishes as the tag's GitHub Release; see check_changelog.
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

# Refuses a CHANGELOG.md that does not say what this version changed for the
# user. release.yml cuts the section out with the same awk and publishes it as
# the tag's GitHub Release, so a section this cannot find is a release with no
# notes. A section is `## <version> - <date>`, holding only Breaking changes,
# Features and Fixes, in that order, none of them empty, and every line under
# them links the commit that made the change.
function check_changelog() {
    local want="$1" section line heading="" items=0 last=-1 idx i sha rc=0
    local -a order=("Breaking changes" "Features" "Fixes")
    if [ ! -f CHANGELOG.md ] ; then
        echo "  FAIL CHANGELOG.md: missing, so ${want} would go out with no release notes" >&2
        return 1
    fi
    section="$(awk -v v="$want" '/^## /{ if (found) exit; found = ($2 == v); next } found' CHANGELOG.md)"
    if ! grep -qE "^## ${want//./\\.} - [0-9]{4}-[0-9]{2}-[0-9]{2}\$" CHANGELOG.md \
        || ! grep -q '^### ' <<< "$section" ; then
        echo "  FAIL CHANGELOG.md: no '## ${want} - <YYYY-MM-DD>' section with a heading under it" >&2
        return 1
    fi
    while IFS= read -r line ; do
        case "$line" in
            '### '*)
                if [ -n "$heading" ] && [ "$items" -eq 0 ] ; then
                    echo "  FAIL CHANGELOG.md: ${want} has an empty '${heading}'; drop the heading" >&2
                    rc=1
                fi
                heading="${line#\#\#\# }"
                items=0
                idx=-1
                for i in "${!order[@]}" ; do
                    [ "${order[$i]}" = "$heading" ] && idx="$i"
                done
                if [ "$idx" -lt 0 ] ; then
                    echo "  FAIL CHANGELOG.md: ${want} has a '${heading}' heading" >&2
                    echo "       the headings are Breaking changes, Features and Fixes" >&2
                    rc=1
                elif [ "$idx" -le "$last" ] ; then
                    echo "  FAIL CHANGELOG.md: ${want} has '${heading}' out of order" >&2
                    echo "       the order is Breaking changes, Features, Fixes" >&2
                    rc=1
                fi
                last="$idx"
                ;;
            '- '*)
                items=$((items + 1))
                sha="$(grep -oE '/commit/[0-9a-f]{40}' <<< "$line" | head -1 | cut -d/ -f3)"
                if [ -z "$sha" ] ; then
                    echo "  FAIL CHANGELOG.md: ${want} has a line that links no commit: ${line}" >&2
                    rc=1
                elif ! git merge-base --is-ancestor "$sha" HEAD 2>/dev/null ; then
                    echo "  FAIL CHANGELOG.md: ${want} links ${sha:0:7}, which is not in this history" >&2
                    rc=1
                fi
                ;;
        esac
    done <<< "$section"
    if [ -n "$heading" ] && [ "$items" -eq 0 ] ; then
        echo "  FAIL CHANGELOG.md: ${want} has an empty '${heading}'; drop the heading" >&2
        rc=1
    fi
    return "$rc"
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

check_changelog "$WANT" || rc=1

if [ "$rc" -ne 0 ] ; then
    echo "==> FAILED - fix every line above, then re-tag" >&2
    exit 1
fi
echo "==> versions agree on ${WANT}, and CHANGELOG.md has its section"
