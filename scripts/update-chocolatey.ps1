# Packs and pushes the Chocolatey package for a release.
#
# Checksums come from the release's own artifacts rather than a local build, so
# the package describes what people will actually download.

param(
    [Parameter(Mandatory=$true)][string]$Version,
    [Parameter(Mandatory=$true)][string]$ApiKey
)

$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

$base = "https://github.com/internetdata/mmdb/releases/download/v$Version"

function Get-ReleaseSha($name) {
    $tmp = Join-Path $env:TEMP $name
    Invoke-WebRequest -Uri "$base/$name" -OutFile $tmp -UseBasicParsing
    $hash = (Get-FileHash -Algorithm SHA256 $tmp).Hash.ToLower()
    Remove-Item $tmp
    return $hash
}

$sha386   = Get-ReleaseSha "mmdb_${Version}_windows_386.zip"
$shaAmd64 = Get-ReleaseSha "mmdb_${Version}_windows_amd64.zip"

$pkg = "chocolatey-packages/mmdb"
$work = Join-Path $env:TEMP "choco-mmdb"
Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
Copy-Item -Recurse $pkg $work

(Get-Content "$work/mmdb.nuspec") `
    -replace '<version>.*</version>', "<version>$Version</version>" `
    -replace '@VSN@', $Version |
    Set-Content "$work/mmdb.nuspec"

(Get-Content "$work/tools/chocolateyinstall.ps1") `
    -replace '@SHA_386@', $sha386 -replace '@SHA_AMD64@', $shaAmd64 |
    Set-Content "$work/tools/chocolateyinstall.ps1"

choco pack "$work/mmdb.nuspec" --outputdirectory $work
choco push "$work/mmdb.$Version.nupkg" --source https://push.chocolatey.org/ --api-key $ApiKey
