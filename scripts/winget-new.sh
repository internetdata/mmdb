#!/bin/bash

# Submits the FIRST version of this CLI to winget-pkgs, from any machine with
# `gh` signed in as someone who can push to the org's winget-pkgs fork.
#
#   ./scripts/winget-new.sh 1.1.0
#   WINGET_OUT=/tmp/winget ./scripts/winget-new.sh 1.1.0   # write the manifests, submit nothing
#   WINGET_PR=12345 ./scripts/winget-new.sh 1.1.0          # put them on that open PR instead
#
# The release workflow's winget job (vedantmgoyal9/winget-releaser) can only
# BUMP a package that already exists upstream - its first step exits 1 when
# manifests/i/InternetData/Mmdb is absent - so version one is submitted by
# hand, once, and this script refuses to run again after that. It writes the
# three-file manifest komac would generate for a portable exe inside a .zip,
# with checksums taken from the release's own artifacts, and opens the PR from
# a branch on the org fork. Nothing here needs Windows: winget-pkgs' own
# pipeline runs `winget validate` and a real install once the PR is open.
# What a moderator asks for before it merges goes onto that PR with WINGET_PR:
# one more commit on its branch, its title and body rewritten to match.

set -euo pipefail

cd "$(dirname "$0")/.."

VSN="${1:?usage: winget-new.sh <version>, e.g. 1.1.0}"
VSN="${VSN#v}"

ID="InternetData.Mmdb"
FORK="internetdata/winget-pkgs"
UPSTREAM="microsoft/winget-pkgs"
# The newest schema winget-pkgs documents: its moderators flag an older one as deprecated.
SCHEMA_VSN="1.28.0"
BRANCH="${ID}-${VSN}"
DIR="manifests/i/InternetData/Mmdb/${VSN}"
BASE="https://github.com/internetdata/mmdb/releases/download/v${VSN}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

if gh api "repos/${UPSTREAM}/contents/manifests/i/InternetData/Mmdb" > /dev/null 2>&1 ; then
    echo "${ID} already exists upstream; later versions are the release workflow's job" >&2
    exit 1
fi

# Architecture as the release archives spell it; winget's spelling is in the manifest below.
function sha_for() {
    local f="mmdb_${VSN}_windows_${1}.zip"
    curl -fsSL -o "${WORK}/${f}" "${BASE}/${f}"
    unzip -l "${WORK}/${f}" | grep -q ' mmdb.exe$' || {
        echo "${f} does not hold mmdb.exe at its root" >&2
        exit 1
    }
    sha256sum "${WORK}/${f}" | cut -d' ' -f1
}

SHA_X86="$(sha_for 386)"
SHA_X64="$(sha_for amd64)"
SHA_ARM64="$(sha_for arm64)"

mkdir -p "${WORK}/${DIR}"

cat > "${WORK}/${DIR}/${ID}.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.version.${SCHEMA_VSN}.schema.json

PackageIdentifier: ${ID}
PackageVersion: ${VSN}
DefaultLocale: en-US
ManifestType: version
ManifestVersion: ${SCHEMA_VSN}
EOF

cat > "${WORK}/${DIR}/${ID}.installer.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.installer.${SCHEMA_VSN}.schema.json

PackageIdentifier: ${ID}
PackageVersion: ${VSN}
InstallerType: zip
NestedInstallerType: portable
NestedInstallerFiles:
  - RelativeFilePath: mmdb.exe
    PortableCommandAlias: mmdb
Commands:
  - mmdb
Installers:
  - Architecture: x86
    InstallerUrl: ${BASE}/mmdb_${VSN}_windows_386.zip
    InstallerSha256: ${SHA_X86}
  - Architecture: x64
    InstallerUrl: ${BASE}/mmdb_${VSN}_windows_amd64.zip
    InstallerSha256: ${SHA_X64}
  - Architecture: arm64
    InstallerUrl: ${BASE}/mmdb_${VSN}_windows_arm64.zip
    InstallerSha256: ${SHA_ARM64}
ManifestType: installer
ManifestVersion: ${SCHEMA_VSN}
EOF

cat > "${WORK}/${DIR}/${ID}.locale.en-US.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.defaultLocale.${SCHEMA_VSN}.schema.json

PackageIdentifier: ${ID}
PackageVersion: ${VSN}
PackageLocale: en-US
Publisher: InternetData
PublisherUrl: https://internetdata.io
PublisherSupportUrl: https://internetdata.io/contact
PrivacyUrl: https://internetdata.io/privacy
PackageName: mmdb
PackageUrl: https://github.com/internetdata/mmdb
License: GPL-3.0
LicenseUrl: https://github.com/internetdata/mmdb/blob/main/LICENSE
ShortDescription: Read, import, export, diff and verify MMDB files
Description: |-
  Read the data for IPs, CIDRs and ranges from an MMDB file, build one from CSV,
  TSV or JSON, export one back to those formats, see the difference between two,
  print a file's metadata, and check that one is well formed.
Moniker: mmdb
Tags:
  - mmdb
  - maxmind
  - ip
  - ip-address
  - database
  - networking
  - cli
ReleaseNotesUrl: https://github.com/internetdata/mmdb/releases/tag/v${VSN}
Documentations:
  - DocumentLabel: Documentation
    DocumentUrl: https://github.com/internetdata/mmdb#readme
ManifestType: defaultLocale
ManifestVersion: ${SCHEMA_VSN}
EOF

if [ -n "${WINGET_OUT:-}" ] ; then
    mkdir -p "${WINGET_OUT}"
    cp -r "${WORK}/manifests" "${WINGET_OUT}/"
    echo "==> wrote ${WINGET_OUT}/${DIR}, submitted nothing"
    exit 0
fi

TITLE="New package: ${ID} version ${VSN}"
BODY="$(cat <<EOF
## 📖 Description

New package: ${ID} version ${VSN} - mmdb, a command line tool to read, import, export, diff and verify MMDB files. A portable \`mmdb.exe\` shipped inside a zip per architecture (x86, x64, arm64), from https://github.com/internetdata/mmdb/releases/tag/v${VSN}; the checksums are those of the published zips.

Authored on Linux, so \`winget validate\` and the install test are left to the pipeline. Later versions are submitted by winget-releaser from that repository's release workflow.

## ✅ Checklist

- [x] Signed the [Contributor License Agreement](https://cla.opensource.microsoft.com)
- [ ] Linked to an issue (if applicable)

## 📦 Manifest Checklist

- [x] Checked that there aren't other open [pull requests](https://github.com/microsoft/winget-pkgs/pulls) for the same manifest update/change
- [x] This PR only modifies one (1) manifest
- [ ] Validated manifest locally with \`winget validate --manifest <path>\` ([validation guide](https://github.com/microsoft/winget-pkgs/blob/master/doc/ValidationFailureGuide.md))
- [ ] Tested manifest locally with \`winget install --manifest <path>\`
- [x] Manifest conforms to the [${SCHEMA_VSN%.0} schema](https://github.com/microsoft/winget-pkgs/tree/master/doc/manifest/schema/${SCHEMA_VSN})
EOF
)"

# One commit on a fork branch, built through the git data API so nobody has to
# clone a repository with a few hundred thousand manifests in it. On an open PR
# it goes on top of the branch, and deletes what the PR added that it does not write.
if [ -n "${WINGET_PR:-}" ] ; then
    PR="$(gh api "repos/${UPSTREAM}/pulls/${WINGET_PR}" \
        -q '[.state, .head.repo.full_name, .head.ref, .head.sha] | join(" ")')"
    read -r PR_STATE PR_REPO BRANCH PARENT <<< "${PR}"
    if [ "${PR_STATE} ${PR_REPO}" != "open ${FORK}" ] ; then
        echo "#${WINGET_PR} is not an open PR from ${FORK}" >&2
        exit 1
    fi
    STALE="$(gh api "repos/${UPSTREAM}/pulls/${WINGET_PR}/files" -q '[.[].filename]')"
    MESSAGE="${ID} version ${VSN}, manifest schema ${SCHEMA_VSN}"
else
    gh repo sync "${FORK}" > /dev/null
    PARENT="$(gh api "repos/${FORK}/git/ref/heads/master" -q .object.sha)"
    STALE='[]'
    MESSAGE="${TITLE}"
fi
BASE_TREE="$(gh api "repos/${FORK}/git/commits/${PARENT}" -q .tree.sha)"

entries='[]'
for f in "${ID}.yaml" "${ID}.installer.yaml" "${ID}.locale.en-US.yaml" ; do
    blob="$(gh api -X POST "repos/${FORK}/git/blobs" -f encoding=base64 \
        -f content="$(base64 -w0 "${WORK}/${DIR}/${f}")" -q .sha)"
    entries="$(jq --arg p "${DIR}/${f}" --arg s "${blob}" \
        '. + [{path: $p, mode: "100644", type: "blob", sha: $s}]' <<< "${entries}")"
done
entries="$(jq --argjson stale "${STALE}" '[.[].path] as $keep
    | . + [$stale[] | select(IN($keep[]) | not) | {path: ., mode: "100644", type: "blob", sha: null}]' \
    <<< "${entries}")"
TREE_SHA="$(jq -n --arg base "${BASE_TREE}" --argjson tree "${entries}" '{base_tree: $base, tree: $tree}' \
    | gh api -X POST "repos/${FORK}/git/trees" --input - -q .sha)"
COMMIT_SHA="$(gh api -X POST "repos/${FORK}/git/commits" -f message="${MESSAGE}" \
    -f tree="${TREE_SHA}" -f "parents[]=${PARENT}" -q .sha)"

if [ -n "${WINGET_PR:-}" ] ; then
    gh api -X PATCH "repos/${FORK}/git/refs/heads/${BRANCH}" -f sha="${COMMIT_SHA}" > /dev/null
    gh api -X PATCH "repos/${UPSTREAM}/pulls/${WINGET_PR}" -f title="${TITLE}" -f body="${BODY}" > /dev/null
    echo "==> #${WINGET_PR} now holds ${ID} ${VSN} on manifest schema ${SCHEMA_VSN}"
    exit 0
fi

gh api -X POST "repos/${FORK}/git/refs" -f ref="refs/heads/${BRANCH}" -f sha="${COMMIT_SHA}" > /dev/null
gh pr create --repo "${UPSTREAM}" --base master --head "${FORK%%/*}:${BRANCH}" \
    --title "${TITLE}" --body "${BODY}"

echo "==> once that PR is merged, set WINGET_TOKEN and every later version is the release workflow's job"
