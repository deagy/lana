#!/usr/bin/env bash
# A local-only release verification helper. It never publishes artifacts.
set -euo pipefail

readonly module_path="github.com/deagy/lana/pkg/version"
readonly binary_name="lana"
readonly dist_dir="${DIST_DIR:-dist}"

notice() {
	printf '%s\n' "$*" >&2
}

skip() {
	notice "SKIPPED: $*"
}

git_value() {
	local value
	if value="$(git "$@" 2>/dev/null)" && [[ -n "$value" ]]; then
		printf '%s' "$value"
		return 0
	fi
	return 1
}

version_value() {
	if [[ -n "${VERSION:-}" ]]; then
		printf '%s' "$VERSION"
		return
	fi
	git_value describe --tags --always --dirty || printf 'dev'
}

commit_value() {
	if [[ -n "${COMMIT:-}" ]]; then
		printf '%s' "$COMMIT"
		return
	fi
	git_value rev-parse --short=12 HEAD || printf 'unknown'
}

source_date_epoch() {
	if [[ -n "${SOURCE_DATE_EPOCH:-}" ]]; then
		printf '%s' "$SOURCE_DATE_EPOCH"
		return
	fi
	git_value log -1 --format=%ct || printf '0'
}

validate_metadata() {
	local value
	for value in "$@"; do
		if [[ ! "$value" =~ ^[0-9A-Za-z._+-]+$ ]]; then
			notice "invalid build metadata value: $value"
			exit 2
		fi
	done
}

build_date() {
	local epoch="$1"
	if date -u -d "@${epoch}" +%Y-%m-%dT%H:%M:%SZ >/dev/null 2>&1; then
		date -u -d "@${epoch}" +%Y-%m-%dT%H:%M:%SZ
	else
		date -u -r "$epoch" +%Y-%m-%dT%H:%M:%SZ
	fi
}

build_binary() {
	local goos="$1" goarch="$2" output="$3"
	local version commit epoch built ldflags
	version="$(version_value)"
	commit="$(commit_value)"
	epoch="$(source_date_epoch)"
	[[ "$epoch" =~ ^[0-9]+$ ]] || { notice "SOURCE_DATE_EPOCH must be a Unix timestamp"; exit 2; }
	validate_metadata "$version" "$commit"
	built="$(build_date "$epoch")"
	ldflags="-s -w -buildid= -X ${module_path}.Version=${version} -X ${module_path}.Commit=${commit} -X ${module_path}.BuildDate=${built}"
	mkdir -p "$(dirname "$output")"
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$output" ./cmd/lana
}

checksum() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$@"
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$@"
	else
		notice "no SHA-256 utility found (need sha256sum or shasum)"
		return 1
	fi
}

check_checksums() {
	local manifest="$1"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum -c "$manifest"
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 -c "$manifest"
	else
		notice "no SHA-256 utility found (need sha256sum or shasum)"
		return 1
	fi
}

package() {
	local version epoch stage_root stage goos goarch archive host_os host_arch completion_binary
	version="$(version_value)"
	epoch="$(source_date_epoch)"
	validate_metadata "$version"
	[[ "$epoch" =~ ^[0-9]+$ ]] || { notice "SOURCE_DATE_EPOCH must be a Unix timestamp"; exit 2; }
	mkdir -p "$dist_dir"
	stage_root="$(mktemp -d "${TMPDIR:-/tmp}/lana-release.XXXXXX")"
	trap 'rm -rf "$stage_root"' RETURN
	host_os="$(go env GOOS)"
	host_arch="$(go env GOARCH)"
	completion_binary="$stage_root/$binary_name"
	build_binary "$host_os" "$host_arch" "$completion_binary"
	for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
		goos="${platform%/*}"
		goarch="${platform#*/}"
		stage="$stage_root/lana_${version}_${goos}_${goarch}"
		mkdir -p "$stage/completions"
		build_binary "$goos" "$goarch" "$stage/$binary_name"
		if [[ "$goos" == "windows" ]]; then
			mv "$stage/$binary_name" "$stage/${binary_name}.exe"
		fi
		for shell in bash zsh fish powershell; do
			"$completion_binary" completion "$shell" > "$stage/completions/$binary_name.$shell"
		done
		archive="$dist_dir/lana_${version}_${goos}_${goarch}.tar.gz"
		go run ./scripts/releasepack --source "$stage" --output "$archive" --epoch "$epoch"
		done
	trap - RETURN
	rm -rf "$stage_root"
	checksums
}

checksums() {
	local -a artifacts
	shopt -s nullglob
	artifacts=("$dist_dir"/lana_*.tar.gz)
	shopt -u nullglob
	if [[ ${#artifacts[@]} -eq 0 ]]; then
		notice "no release archives found in $dist_dir"
		return 1
	fi
	(
		cd "$dist_dir"
		LC_ALL=C checksum lana_*.tar.gz
	) > "$dist_dir/checksums.txt"
	notice "wrote $dist_dir/checksums.txt"
}

verify_artifacts() {
	local manifest="$dist_dir/checksums.txt" entries
	[[ -s "$manifest" ]] || { notice "missing checksum manifest: $manifest"; return 1; }
	(
		cd "$dist_dir"
		check_checksums checksums.txt
		for archive in lana_*.tar.gz; do
			entries="$(tar -tzf "$archive")"
			grep -Fqx 'lana' <<<"$entries" || { notice "$archive does not contain lana"; exit 1; }
			for shell in bash zsh fish powershell; do
				grep -Fqx "completions/lana.$shell" <<<"$entries" || { notice "$archive lacks $shell completion"; exit 1; }
			done
		done
	)
	notice "release artifacts and checksums verified"
}

validate_install() {
	local goos goarch version archive temp_root extracted binary completion
	goos="$(go env GOOS)"
	goarch="$(go env GOARCH)"
	version="$(version_value)"
	archive="$dist_dir/lana_${version}_${goos}_${goarch}.tar.gz"
	[[ -f "$archive" ]] || { notice "native archive not found: $archive"; return 1; }
	temp_root="$(mktemp -d "${TMPDIR:-/tmp}/lana-install.XXXXXX")"
	trap 'rm -rf "$temp_root"' RETURN
	tar -xzf "$archive" -C "$temp_root"
	mkdir -p "$temp_root/install/bin" "$temp_root/install/completions"
	binary="$temp_root/lana"
	cp "$binary" "$temp_root/install/bin/lana"
	chmod +x "$temp_root/install/bin/lana"
	"$temp_root/install/bin/lana" --help >/dev/null
	"$temp_root/install/bin/lana" --version | grep -Fq "lana version $version"
	for shell in bash zsh fish powershell; do
		completion="$temp_root/install/completions/lana.$shell"
		cp "$temp_root/completions/lana.$shell" "$completion"
		test -s "$completion"
		diff -q "$completion" <("$temp_root/install/bin/lana" completion "$shell") >/dev/null
	done
	trap - RETURN
	rm -rf "$temp_root"
	notice "native archive installation and completion generation verified"
}

reproducibility() {
	local goos goarch temp_root epoch version commit
	goos="$(go env GOOS)"
	goarch="$(go env GOARCH)"
	temp_root="$(mktemp -d "${TMPDIR:-/tmp}/lana-repro.XXXXXX")"
	trap 'rm -rf "$temp_root"' RETURN
	epoch="$(source_date_epoch)"
	version="$(version_value)"
	commit="$(commit_value)"
	SOURCE_DATE_EPOCH="$epoch" VERSION="$version" COMMIT="$commit" build_binary "$goos" "$goarch" "$temp_root/first/lana"
	SOURCE_DATE_EPOCH="$epoch" VERSION="$version" COMMIT="$commit" build_binary "$goos" "$goarch" "$temp_root/second/lana"
	cmp -s "$temp_root/first/lana" "$temp_root/second/lana" || { notice "identical inputs produced different binaries"; return 1; }
	trap - RETURN
	rm -rf "$temp_root"
	notice "native build is reproducible for the selected metadata"
}

security_scan() {
	if command -v govulncheck >/dev/null 2>&1; then
		govulncheck ./...
	else
		skip "govulncheck is unavailable; install golang.org/x/vuln/cmd/govulncheck to enable vulnerability scanning"
	fi
}

license_scan() {
	if command -v go-licenses >/dev/null 2>&1; then
		go-licenses check ./...
	else
		skip "go-licenses is unavailable; install github.com/google/go-licenses to enable license scanning"
	fi
}

sbom() {
	local output="$dist_dir/lana.sbom.cdx.json"
	if command -v syft >/dev/null 2>&1; then
		mkdir -p "$dist_dir"
		syft dir:. -o "cyclonedx-json=$output"
		notice "wrote $output"
	else
		skip "syft is unavailable; install syft to generate a CycloneDX SBOM"
	fi
}

usage() {
	cat <<'EOF'
Usage: scripts/release.sh <command>

Commands: package, checksums, verify-artifacts, validate-install,
          reproducibility, security-scan, license-scan, sbom
EOF
}

case "${1:-}" in
	package) package ;;
	checksums) checksums ;;
	verify-artifacts) verify_artifacts ;;
	validate-install) validate_install ;;
	reproducibility) reproducibility ;;
	security-scan) security_scan ;;
	license-scan) license_scan ;;
	sbom) sbom ;;
	*) usage; exit 2 ;;
esac
