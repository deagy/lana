SHELL := /usr/bin/env bash

GO ?= go
DIST_DIR ?= dist

.PHONY: all fmt fmt-check vet test build package checksums verify-artifacts validate-install reproducibility security-scan license-scan sbom ci clean

all: build

fmt:
	@$(GO) fmt ./...

fmt-check:
	@unformatted="$$(gofmt -l $$(find . -type f -name '*.go' -not -path './vendor/*'))"; \
	if [[ -n "$$unformatted" ]]; then \
		echo "Go files need formatting:" >&2; \
		echo "$$unformatted" >&2; \
		exit 1; \
	fi

vet:
	@$(GO) vet ./...

test:
	@$(GO) test ./...

build:
	@$(MAKE) --no-print-directory package GOOS="$$(go env GOOS)" GOARCH="$$(go env GOARCH)"

package:
	@DIST_DIR="$(DIST_DIR)" ./scripts/release.sh package

checksums:
	@DIST_DIR="$(DIST_DIR)" ./scripts/release.sh checksums

verify-artifacts:
	@DIST_DIR="$(DIST_DIR)" ./scripts/release.sh verify-artifacts

validate-install:
	@DIST_DIR="$(DIST_DIR)" ./scripts/release.sh validate-install

reproducibility:
	@DIST_DIR="$(DIST_DIR)" ./scripts/release.sh reproducibility

security-scan:
	@./scripts/release.sh security-scan

license-scan:
	@./scripts/release.sh license-scan

sbom:
	@DIST_DIR="$(DIST_DIR)" ./scripts/release.sh sbom

ci: fmt-check vet test reproducibility package sbom security-scan license-scan verify-artifacts validate-install

clean:
	@rm -rf "$(DIST_DIR)"
