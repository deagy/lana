# Release Checklist

Use this checklist before releasing a new version.

## Pre-Release (1 week before)

- [ ] Review open issues and PRs
- [ ] Plan features and fixes for this release
- [ ] Estimate version bump (major.minor.patch)
- [ ] Create release branch: `git checkout -b release/vX.Y.Z`

## Code Preparation (3 days before)

- [ ] Run all tests: `make test`
- [ ] Check coverage: `make cover`
- [ ] Format code: `make fmt`
- [ ] Lint code: `make lint`
- [ ] Fix any warnings or errors
- [ ] Update CHANGELOG.md with new features/fixes
- [ ] Update version in hardcoded strings (if any)
- [ ] Commit changes with message: "chore: Prepare release vX.Y.Z"

## Documentation (2 days before)

- [ ] Update README.md if needed
- [ ] Review USER_GUIDE.md for accuracy
- [ ] Update API.md with any new APIs
- [ ] Add release notes to docs/RELEASES.md
- [ ] Verify examples still work
- [ ] Check for broken links in docs

## Testing (1 day before)

- [ ] Run full test suite: `go test ./...`
- [ ] Test on Linux: `make release`
- [ ] Test on macOS (if available): `GOOS=darwin make release`
- [ ] Test on Windows (if available): `GOOS=windows make release`
- [ ] Manual testing of key features:
  - [ ] `lana chat` works
  - [ ] `lana run` works
  - [ ] File operations work
  - [ ] Git operations work
  - [ ] Sessions persist
  - [ ] Config management works

## Release Day

### Build & Tag

- [ ] Build release binaries: `make release-all`
- [ ] Test each binary:
  - [ ] `./dist/lana-linux-amd64 version`
  - [ ] `./dist/lana-darwin-amd64 version`
  - [ ] `./dist/lana-windows-amd64.exe version`
- [ ] Create git tag: `git tag -a vX.Y.Z -m "Release vX.Y.Z"`
- [ ] Push tag: `git push origin vX.Y.Z`

### Create Release

On GitHub:
- [ ] Go to Releases page
- [ ] Click "Create release"
- [ ] Select tag vX.Y.Z
- [ ] Title: "Lana vX.Y.Z"
- [ ] Add release notes from CHANGELOG
- [ ] Attach binaries from `dist/`
- [ ] Mark as latest release
- [ ] Publish release

### Post-Release

- [ ] Merge release branch back to main
- [ ] Close related GitHub issues
- [ ] Announce on social media/forums
- [ ] Update website (if exists)

## Version Numbering

Follow [Semantic Versioning](https://semver.org/):

- **Major (X.0.0)**: Breaking changes (API changes, removed features)
- **Minor (1.Y.0)**: New features (backward compatible)
- **Patch (1.0.Z)**: Bug fixes (backward compatible)

Examples:
- `v0.1.0` → `v0.2.0` — New feature
- `v0.1.0` → `v0.1.1` — Bug fix
- `v0.1.0` → `v1.0.0` — Breaking change / first stable release

## Rollback Plan

If a release has critical issues:

1. Tag as `vX.Y.Z-rc1` for release candidate
2. Document issue in GitHub issue
3. Create hotfix branch: `git checkout -b hotfix/vX.Y.Z+1`
4. Fix the issue
5. Bump patch version
6. Release vX.Y.Z+1

## Release Notes Template

```markdown
# Lana vX.Y.Z

**Release Date:** 2025-08-12

## Features

- Feature 1 description
- Feature 2 description

## Improvements

- Improvement 1
- Improvement 2

## Bug Fixes

- Fixed issue #123: description
- Fixed issue #456: description

## Breaking Changes

If any:
- Change 1

## Installation

```bash
# Download the binary for your platform
# Linux x86_64
wget https://github.com/deagy/lana/releases/download/vX.Y.Z/lana-linux-amd64
chmod +x lana-linux-amd64

# Or build from source
git clone https://github.com/deagy/lana
cd lana
git checkout vX.Y.Z
make install
```

## Changelog

Full changelog: [CHANGELOG.md](../CHANGELOG.md)
```

## Binary Distribution

All binaries should be:
- [ ] Built with version info: `make release-all`
- [ ] Tested on target platform
- [ ] Named with pattern: `lana-{os}-{arch}`
- [ ] Compressed (optional): `gzip lana-*`
- [ ] Uploaded to GitHub release

Platform matrix:
- linux-amd64 (Intel/AMD 64-bit)
- linux-arm64 (ARM 64-bit)
- darwin-amd64 (macOS Intel)
- darwin-arm64 (macOS Apple Silicon)
- windows-amd64 (Windows 64-bit)

## Verification

Users should be able to verify release integrity:

```bash
# Check version
lana version
# Output should show vX.Y.Z

# Check binary works
lana chat --help
lana run --help
```

## Documentation Updates

Each release should update:
- [ ] CHANGELOG.md (what changed)
- [ ] README.md (if setup changed)
- [ ] USER_GUIDE.md (if commands changed)
- [ ] API.md (if APIs changed)
- [ ] docs/RELEASES.md (release notes)

## Support Period

Each release is supported until the next major release or 6 months, whichever is shorter.

Security fixes can be backported to previous versions if critical.
