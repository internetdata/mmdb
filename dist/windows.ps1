# Installs mmdb on Windows.
#
#   iwr -useb https://github.com/internetdata/mmdb/releases/latest/download/windows.ps1 | iex

$ErrorActionPreference = "Stop"

$vsn = if ($env:VSN) { $env:VSN } else { "1.0.1" }
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    "x86"   { "386" }
    default { throw "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
}

$zip = "mmdb_${vsn}_windows_${arch}.zip"
$url = "https://github.com/internetdata/mmdb/releases/download/v${vsn}/${zip}"

# Per-user, so this needs no elevation. Machine-wide installs should use the
# Chocolatey or WinGet package instead.
$dest = Join-Path $env:LOCALAPPDATA "Programs\mmdb"
New-Item -ItemType Directory -Force -Path $dest | Out-Null

$tmp = Join-Path $env:TEMP $zip
Write-Host "downloading $zip..."
Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing
Expand-Archive -Path $tmp -DestinationPath $dest -Force
Remove-Item $tmp

# Only append to PATH when it is not already there, or repeated runs grow it.
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$dest*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$dest", "User")
    Write-Host "added $dest to your PATH; open a new terminal to pick it up."
}

Write-Host ""
Write-Host "installed. run 'mmdb --help'."
