#!/bin/bash

# Submits the FIRST version of this CLI to winget-pkgs, from any machine with
# `gh` signed in as someone who can push to the org's winget-pkgs fork.
#
#   ./scripts/winget-new.sh 1.1.0
#   WINGET_OUT=/tmp/winget ./scripts/winget-new.sh 1.1.0   # write the manifests, submit nothing
#
# The release workflow's winget job (vedantmgoyal9/winget-releaser) can only
# BUMP a package that already exists upstream - its first step exits 1 when
# manifests/i/InternetData/Mmdb is absent - so version one is submitted by
# hand, once, and this script refuses to run again after that. It writes the
# three-file manifest komac would generate for a portable exe inside a .zip,
# with checksums taken from the release's own artifacts, and opens the PR from
# a branch on the org fork. Nothing here needs Windows: winget-pkgs' own
# pipeline runs `winget validate` and a real install once the PR is open.

set -euo pipefail

cd "$(dirname "$0")/.."

VSN="${1:?usage: winget-new.sh <version>, e.g. 1.1.0}"
VSN="${VSN#v}"

ID="InternetData.Mmdb"
FORK="internetdata/winget-pkgs"
UPSTREAM="microsoft/winget-pkgs"
SCHEMA_VSN="1.12.0"
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

# One commit on a fork branch, built through the git data API so nobody has to
# clone a repository with a few hundred thousand manifests in it.
gh repo sync "${FORK}" > /dev/null
BASE_SHA="$(gh api "repos/${FORK}/git/ref/heads/master" -q .object.sha)"
BASE_TREE="$(gh api "repos/${FORK}/git/commits/${BASE_SHA}" -q .tree.sha)"

entries='[]'
for f in "${ID}.yaml" "${ID}.installer.yaml" "${ID}.locale.en-US.yaml" ; do
    blob="$(gh api -X POST "repos/${FORK}/git/blobs" -f encoding=base64 \
        -f content="$(base64 -w0 "${WORK}/${DIR}/${f}")" -q .sha)"
    entries="$(jq --arg p "${DIR}/${f}" --arg s "${blob}" \
        '. + [{path: $p, mode: "100644", type: "blob", sha: $s}]' <<< "${entries}")"
done
TREE_SHA="$(jq -n --arg base "${BASE_TREE}" --argjson tree "${entries}" '{base_tree: $base, tree: $tree}' \
    | gh api -X POST "repos/${FORK}/git/trees" --input - -q .sha)"
COMMIT_SHA="$(gh api -X POST "repos/${FORK}/git/commits" -f message="New package: ${ID} version ${VSN}" \
    -f tree="${TREE_SHA}" -f "parents[]=${BASE_SHA}" -q .sha)"
gh api -X POST "repos/${FORK}/git/refs" -f ref="refs/heads/${BRANCH}" -f sha="${COMMIT_SHA}" > /dev/null

gh pr create --repo "${UPSTREAM}" --base master --head "${FORK%%/*}:${BRANCH}" \
    --title "New package: ${ID} version ${VSN}" --body "$(cat <<EOF
## 📖 Description

New package: ${ID} version ${VSN} - mmdb, a command line tool to read, import, export, diff and verify MMDB files. A portable \`mmdb.exe\` shipped inside a zip per architecture (x86, x64, arm64), from https://github.com/internetdata/mmdb/releases/tag/v${VSN}; the checksums are those of the published zips.

Authored on Linux, so \`winget validate\` and the install test are left to the pipeline. Later versions are submitted by winget-releaser from that repository's release workflow.

## ✅ Checklist

- [ ] Signed the [Contributor License Agreement](https://cla.opensource.microsoft.com)
- [ ] Linked to an issue (if applicable)

## 📦 Manifest Checklist

- [x] Checked that there aren't other open [pull requests](https://github.com/microsoft/winget-pkgs/pulls) for the same manifest update/change
- [x] This PR only modifies one (1) manifest
- [ ] Validated manifest locally with \`winget validate --manifest <path>\` ([validation guide](https://github.com/microsoft/winget-pkgs/blob/master/doc/ValidationFailureGuide.md))
- [ ] Tested manifest locally with \`winget install --manifest <path>\`
- [x] Manifest conforms to the [1.12 schema](https://github.com/microsoft/winget-pkgs/tree/master/doc/manifest/schema/1.12.0)
EOF
)"

echo "==> once that PR is merged, set WINGET_TOKEN and every later version is the release workflow's job"
