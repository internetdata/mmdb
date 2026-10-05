# Changelog

What each release changed for you, newest first. Each line is a commit's summary, linked to its full description and diff. 1.0.0, the first release, is described by the commits up to its tag.

## 1.2.1 - 2026-10-05

### Fixes

- Take maxminddb-golang 2.7.0: verify accepts a record pointing into another ([`5aea249`](https://github.com/internetdata/mmdb/commit/5aea24941f091aae3fc8e5e643d8192bcef3710d))
- macos.sh: install the arm64 binary from a shell running under Rosetta ([`cf658d4`](https://github.com/internetdata/mmdb/commit/cf658d41ddf0988c8672c046aa40d78cee355ff9))

## 1.2.0 - 2026-10-03

### Features

- Add import --dry-run, which sizes a build without running it ([`c5717d1`](https://github.com/internetdata/mmdb/commit/c5717d18610313cc66f0753a2f0236d035271891))

## 1.1.0 - 2026-10-02

### Features

- Add compress, which shrinks a legacy mmdb file, one not written by mmdbwriter v2 ([`1b6dfc7`](https://github.com/internetdata/mmdb/commit/1b6dfc716739cbc122a0a3fab8e05df01e3f6be2))

## 1.0.1 - 2026-09-29

### Fixes

- Take pflag 1.0.10: print a flag error once, on stderr alone ([`e34d847`](https://github.com/internetdata/mmdb/commit/e34d847560798823c5e681f7372f920bee23a59d))
