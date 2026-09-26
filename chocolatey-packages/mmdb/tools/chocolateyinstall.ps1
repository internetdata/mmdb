$ErrorActionPreference = 'Stop'

$packageName = 'mmdb'
$version     = $env:ChocolateyPackageVersion
$base        = "https://github.com/internetdata/mmdb/releases/download/v$version"

# Checksums are substituted by scripts/update-chocolatey.ps1 at pack time, from
# the release's own artifacts.
Install-ChocolateyZipPackage `
    -PackageName    $packageName `
    -Url            "$base/mmdb_${version}_windows_386.zip" `
    -Url64bit       "$base/mmdb_${version}_windows_amd64.zip" `
    -UnzipLocation  (Split-Path -Parent $MyInvocation.MyCommand.Definition) `
    -Checksum       '@SHA_386@' `
    -ChecksumType   'sha256' `
    -Checksum64     '@SHA_AMD64@' `
    -ChecksumType64 'sha256'
