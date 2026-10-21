# Changelog

What each release changed for you, newest first. Each line is a commit's summary, linked to its full description and diff. 1.0.0, the first release, is described by the commits up to its tag.

## 1.7.0 - 2026-10-21

### Features

- import: write the smallest record size that fits ([`7ab9330`](https://github.com/internetdata/mmdb/commit/7ab93300dda8921e375ec629349c8dc220a8ef2f))

### Fixes

- Build every release on Go 1.27.3 ([`bc8c7e7`](https://github.com/internetdata/mmdb/commit/bc8c7e7587b3eaae373f13f848daebce93221493))

## 1.6.1 - 2026-10-19

### Fixes

- read: keep a list's first address after a byte order mark ([`ef54520`](https://github.com/internetdata/mmdb/commit/ef54520e430afb34a697908f9517ef803307ec2b))
- Take maxminddb-golang 2.8.0, go-colorable 0.1.16 and x/sys 0.49.0 ([`6eebea3`](https://github.com/internetdata/mmdb/commit/6eebea382737f84595138b65534b513a50589b2f))

## 1.6.0 - 2026-10-15

### Features

- Add --backup, which keeps the file import or compress replaces ([`620374d`](https://github.com/internetdata/mmdb/commit/620374d12f6894813aaa411663ccd0a42fe0747a))

## 1.5.0 - 2026-10-13

### Features

- import --json: read an array, and report a bad record instead of crashing ([`6727757`](https://github.com/internetdata/mmdb/commit/67277576ba25243d974b37802e228abbbb964624))
- Take mmdbwriter v2 as of 2026-10-07: imports run up to twice as fast ([`02c54f6`](https://github.com/internetdata/mmdb/commit/02c54f694ec3d954eae942dcb84cecb2d5f89aa8))

## 1.4.0 - 2026-10-13

### Fixes

- Keep the old output whole when an import or compress fails ([`e5ef0a8`](https://github.com/internetdata/mmdb/commit/e5ef0a8c912b5e75cd954f8b0b07aa856911bed7))

## 1.3.1 - 2026-10-12

### Fixes

- Offer the options again after a flag that takes no value ([`d5684a6`](https://github.com/internetdata/mmdb/commit/d5684a6476dcb3aaea786e29a2cffadf6e5936fb))
- Build every release on Go 1.27.2, pinned in all four places ([`c844123`](https://github.com/internetdata/mmdb/commit/c8441233ab7c57c3303d1bca2b36b46595661ef9))

## 1.3.0 - 2026-10-07

### Fixes

- Exit 1 when a command fails, and when verify finds the file invalid ([`c3ad056`](https://github.com/internetdata/mmdb/commit/c3ad056f2986088ea478df73df88ab3676c6ed74))
- read: print the help when no mmdb file is named, whatever the flags ([`e817bdd`](https://github.com/internetdata/mmdb/commit/e817bdd8e46f1a303e27cdbc8b1a0a7e45aab1cc))

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
