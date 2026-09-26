# [<img src="https://s3.internetdata.io/internetdata-public/brand/mark.svg" alt="InternetData" height="28"/>](https://internetdata.io/) InternetData `mmdb`

[![release](https://img.shields.io/github/v/release/internetdata/mmdb)](https://github.com/internetdata/mmdb/releases)
[![license](https://img.shields.io/github/license/internetdata/mmdb)](LICENSE)

A free command line utility for MMDB files, maintained by [InternetData](https://internetdata.io). It works with any MMDB file, whoever built it, and needs no account or API key.

With it you can:

- Read data for IPs in an MMDB file.
- Import data in non-MMDB format into MMDB.
- Export data from MMDB format into non-MMDB format.
- See the difference between two MMDB files.
- Print the metadata of an MMDB file.
- Check that an MMDB file is not corrupted or invalid.

## Getting Started

### macOS

```bash
brew trust internetdata/tap
brew tap internetdata/tap
brew install mmdb
```

Homebrew won't load a formula from a third-party tap until you trust it. Skip that first line and it fails with `invalid syntax in tap!`, which is misleading: the formula is fine. `brew trust` arrived in Homebrew 7, so run `brew update` first if it comes back as an unknown command. The same three lines work with Homebrew on Linux.

`mmdb` needs macOS 13 Ventura or later.

### Debian / Ubuntu

Install from our apt repository, which keeps `mmdb` up to date with `apt upgrade`:

```bash
echo "deb [trusted=yes] https://apt.internetdata.io/mmdb/ /" | sudo tee /etc/apt/sources.list.d/mmdb.list
sudo apt update && sudo apt install mmdb
```

Or install a single `.deb` without the repository:

```bash
curl -Ls https://github.com/internetdata/mmdb/releases/latest/download/deb.sh | sh
```

### Windows

Install for the current user, which needs no admin rights:

```powershell
iwr -useb https://github.com/internetdata/mmdb/releases/latest/download/windows.ps1 | iex
```

### Docker

```bash
docker run --rm -v "$PWD:/data" -w /data ghcr.io/internetdata/mmdb metadata location.mmdb
```

### Using `go install`

```bash
go install github.com/internetdata/mmdb@latest
```

### Using `curl` / `wget`

Binaries are published for 23 platform and architecture pairs on the [releases page](https://github.com/internetdata/mmdb/releases). Pick yours:

```bash
# Linux amd64; for Windows use ".zip" instead of ".tar.gz"
curl -LO https://github.com/internetdata/mmdb/releases/download/v1.0.0/mmdb_1.0.0_linux_amd64.tar.gz
tar -xzf mmdb_1.0.0_linux_amd64.tar.gz
sudo mv mmdb /usr/local/bin/
```

macOS has a one-line installer that picks the right architecture for you:

```bash
curl -Ls https://github.com/internetdata/mmdb/releases/latest/download/macos.sh | sh
```

Note that the binaries are not code-signed, so macOS Gatekeeper will ask before running one the first time, and Windows SmartScreen may warn.

### From source

```bash
git clone https://github.com/internetdata/mmdb
cd mmdb
./scripts/build.sh
```

The result lands in `build/`. Go 1.27 or newer.

## Quick Start

This will help you quickly get started with the `mmdb` CLI.

### Reading

You can read from MMDB files in various different ways - as individual IPs, CIDRs or IP ranges, coming from the command line as arguments, or from files, or from stdin.

Pretty JSON format:

```bash
$ mmdb read -f json-pretty 8.8.8.8 location.mmdb
{
  "city": "Mountain View",
  "country": "US",
  "geoname_id": "5375480",
  "latitude": "37.4056",
  "longitude": "-122.0775",
  "postalcode": "94043",
  "region": "California",
  "timezone": "America/Los_Angeles"
}
```

CSV format:

```bash
$ mmdb read -f csv 8.8.8.8 location.mmdb
ip,city,country,geoname_id,latitude,longitude,postalcode,region,timezone
8.8.8.8,Mountain View,US,5375480,37.4056,-122.0775,94043,California,America/Los_Angeles
```

TSV format:

```bash
$ mmdb read -f tsv 8.8.8.8 location.mmdb
ip	city	country	geoname_id	latitude	longitude	postalcode	region	timezone
8.8.8.8	Mountain View	US	5375480	37.4056	-122.0775	94043	California	America/Los_Angeles
```

Via a file:

```bash
$ cat ips.txt
8.8.8.8
8.8.8.0/31
8.8.8.0-8.8.8.1
8.8.8.0,8.8.8.1

$ mmdb read ips.txt location.mmdb | sort -u
{"city":"Mountain View","country":"US","geoname_id":"5375480","latitude":"37.4056","longitude":"-122.0775","postalcode":"94043","region":"California","timezone":"America/Los_Angeles"}
```

Via stdin:

```bash
$ echo 8.8.8.8 | mmdb read location.mmdb
{"city":"Mountain View","country":"US","geoname_id":"5375480","latitude":"37.4056","longitude":"-122.0775","postalcode":"94043","region":"California","timezone":"America/Los_Angeles"}
```

Multiple inputs are also possible - these all return the same thing:

```bash
$ echo -e '8.8.8.8\n1.2.3.4' | mmdb read location.mmdb
$ mmdb read 8.8.8.8 1.2.3.4 location.mmdb
{"city":"Mountain View","country":"US","geoname_id":"5375480","latitude":"37.4056","longitude":"-122.0775","postalcode":"94043","region":"California","timezone":"America/Los_Angeles"}
{"city":"Brisbane","country":"AU","geoname_id":"2174003","latitude":"-27.48203","longitude":"153.01358","postalcode":"4101","region":"Queensland","timezone":"Australia/Brisbane"}
```

Can check CIDRs and ranges - these will all return the same thing:

```bash
$ mmdb read 8.8.8.0/31 location.mmdb
$ mmdb read 8.8.8.0-8.8.8.1 location.mmdb
$ mmdb read 8.8.8.0,8.8.8.1 location.mmdb
{"city":"Mountain View","country":"US","geoname_id":"5375480","latitude":"37.4056","longitude":"-122.0775","postalcode":"94043","region":"California","timezone":"America/Los_Angeles"}
{"city":"Mountain View","country":"US","geoname_id":"5375480","latitude":"37.4056","longitude":"-122.0775","postalcode":"94043","region":"California","timezone":"America/Los_Angeles"}
{"city":"Mountain View","country":"US","geoname_id":"5375480","latitude":"37.4056","longitude":"-122.0775","postalcode":"94043","region":"California","timezone":"America/Los_Angeles"}
```

### Importing

Importing allows taking in files as CSV/TSV/JSON, and outputting an MMDB file.

Importing is one of the most powerful/flexible features in `mmdb`. However, we only allow strings throughout the data at the current time.

See `mmdb import --help` for full details on usage.

Here are some basic examples:

```bash
# basic CSV importing into MMDB.
$ mmdb import --in data.csv --out data.mmdb

# generate MMDB from a TSV file containing IPv4 data.
$ cat data.tsv | mmdb import --ip 4 --tsv --out data.mmdb

# don't include the implicit `network` field in the output MMDB:
$ mmdb import --no-network --in data.csv --out data.mmdb

# generate an MMDB without any fields, just IP ranges that meet a criteria.
$ mmdb import \
    --size 24 --no-fields --ip 4 \
    --in anycast.csv --out anycast.mmdb
```

### Exporting

Exporting allows taking in an MMDB file and outputting CSV/TSV/JSON.

See `mmdb export --help` for full details on usage.

A network holding a single address is written without its prefix length, as
`8.8.8.8` rather than `8.8.8.8/32`. Pass `--cidr-only` to always get CIDR form.

```bash
# basic export.
$ mmdb export data.mmdb data.csv

# basic export without a header.
$ mmdb export --no-header data.mmdb data.csv

# just see the number of entries it'd output.
$ mmdb export --no-header --format csv data.mmdb | wc -l
```

### Metadata

You can retrieve data in the `metadata` section of the MMDB file using the `metadata` subcommand.

Pretty format:

```bash
$ mmdb metadata location.mmdb
- Binary Format 2.0
- Database Type internetdata location.mmdb
- IP Version    4
- Record Size   32
- Node Count    123456789
- Description
    en internetdata location.mmdb
- Languages     en
- Build Epoch   123456789
```

JSON format:

```bash
$ mmdb metadata -f json location.mmdb
{
    "binary_format": "2.0",
    "db_type": "internetdata location.mmdb",
    "ip": 4,
    "record_size": 32,
    "node_count": 123456789,
    "description": {
        "en": "internetdata location.mmdb"
    },
    "languages": [
        "en"
    ],
    "build_epoch": 123456789
}
```

### Verification

You can verify if a MMDB file is correctly structured with the `verify` subcommand:

```bash
$ mmdb verify location.mmdb
valid
```

Let's force it to be invalid and check again:

```bash
$ cp location.mmdb location-tmp.mmdb
$ cat location.mmdb >> location-tmp.mmdb
$ mmdb verify location-tmp.mmdb
invalid: received decoding error (the MaxMind DB file's data section contains bad data (uint16 size of 11)) at offset of 13825601
```

## Auto-Completion

Auto-completion is supported for at least the following shells:

```
bash
zsh
fish
```

NOTE: it may work for other shells as well because the implementation is in Golang and is not necessarily shell-specific.

### Installation

Installing auto-completions is as simple as running one command (works for `bash`, `zsh` and `fish` shells):

```bash
mmdb completion install
```

If you want to customize the installation process (e.g. in case the auto-installation doesn't work as expected), you can request the actual completion script for each shell:

```bash
# get bash completion script
mmdb completion bash

# get zsh completion script
mmdb completion zsh

# get fish completion script
mmdb completion fish
```

### Shell not listed?

If your shell is not listed here, you can [open an issue](https://github.com/internetdata/mmdb/issues).

Note that as long as the `COMP_LINE` environment variable is provided to the binary itself, it will output completion results. So if your shell provides a way to pass `COMP_LINE` on auto-completion attempts to a binary, then have your shell do that with the `mmdb` binary itself (or any of our binaries).

## Color Output

### Disabling Color Output

All our CLIs respect either the `--nocolor` flag or the [`NO_COLOR`](https://no-color.org/) environment variable to disable color output.

### Color on Windows

To enable color support for the Windows command prompt, run the following to enable [`Console Virtual Terminal Sequences`](https://docs.microsoft.com/en-us/windows/console/console-virtual-terminal-sequences).

```cmd
REG ADD HKCU\CONSOLE /f /v VirtualTerminalLevel /t REG_DWORD /d 1
```

You can disable this by running the following:

```cmd
REG DELETE HKCU\CONSOLE /f /v VirtualTerminalLevel
```

## Other Tools

To download InternetData's own databases, MMDB included, use the [InternetData CLI](https://github.com/internetdata/cli) or one of our client libraries for PHP, Python, Go, Java, Ruby and more. See our GitHub at https://github.com/internetdata for all of them.

## About InternetData

IP, ASN and Domain data to reveal unique insights about the internet. APIs, Databases and Live Feeds available.

[<img src="https://s3.internetdata.io/internetdata-public/brand/mark.svg" alt="InternetData" height="64"/>](https://internetdata.io/)

## License

This project is licensed under the [GNU General Public License v3.0](LICENSE).
