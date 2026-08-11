# Local build and release verification

This repository's release tooling creates local Linux and macOS archives only.
It does not publish a release, upload an artifact, sign an artifact, or contact
an external service.

Run the full local verification baseline with:

```sh
make ci
```

The command checks Go formatting, `go vet`, tests, reproducible native builds,
cross-platform builds for Linux and macOS (`amd64` and `arm64`), archive
contents, SHA-256 checksums, and native archive installation. It also checks
that each installed completion artifact matches fresh output from `lana
completion`.

Release archives and `dist/checksums.txt` are written to `dist/`. The generated
binary receives deterministic version metadata. Set these values explicitly
for a release candidate:

```sh
SOURCE_DATE_EPOCH=1735689600 VERSION=v0.1.0 COMMIT=0123456789ab make package
```

When unset, the script uses the current Git description, commit, and commit
timestamp; a source tree without commits uses the deterministic defaults
`dev`, `unknown`, and Unix epoch `0`.

Optional supply-chain checks are graceful when their tools are not installed:

- `make security-scan` runs `govulncheck ./...` when `govulncheck` is on `PATH`.
- `make license-scan` runs `go-licenses check ./...` when `go-licenses` is on
  `PATH`.
- `make sbom` writes `dist/lana.sbom.cdx.json` with `syft` when it is on `PATH`.

An unavailable optional tool is reported as `SKIPPED` and does not make local
builds fail. Install the relevant tool before a release candidate if that scan
is required.
