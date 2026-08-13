.PHONY: build test lint fmt clean help release release-all install

# Versioning
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD)
LDFLAGS := -ldflags "-X github.com/deagy/lana/internal/cmd.Version=$(VERSION) -X github.com/deagy/lana/internal/cmd.Commit=$(COMMIT)"

# Build the binary
build:
	@echo "Building lana..."
	go build $(LDFLAGS) -o lana ./cmd/lana

# Run all tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
cover:
	@echo "Running tests with coverage..."
	go test -cover ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	go tool goimports -w .

# Run linters
lint:
	@echo "Running linters..."
	go vet ./...
	@which golangci-lint > /dev/null && golangci-lint run ./... || echo "golangci-lint not installed"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f lana
	go clean

# Run the CLI
run: build
	./lana version

# Run specific command
run-cmd: build
	./lana $(CMD)

# Build release binaries for all platforms
release-all: clean
	@echo "Building release binaries..."
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/lana-linux-amd64 ./cmd/lana
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/lana-linux-arm64 ./cmd/lana
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/lana-darwin-amd64 ./cmd/lana
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/lana-darwin-arm64 ./cmd/lana
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/lana-windows-amd64.exe ./cmd/lana
	@echo "Binaries built in dist/"
	ls -lh dist/

# Build release for current platform
release: clean
	@echo "Building release for current platform..."
	mkdir -p dist
	go build $(LDFLAGS) -o dist/lana ./cmd/lana
	@echo "Binary: dist/lana"

# Install to $GOPATH/bin
install: build
	@echo "Installing lana..."
	go install $(LDFLAGS) ./cmd/lana
	@echo "Installed to $$(go env GOPATH)/bin/lana"

# Show help
help:
	@echo "Available targets:"
	@echo "  build       - Build the lana binary"
	@echo "  test        - Run all tests"
	@echo "  cover       - Run tests with coverage"
	@echo "  fmt         - Format code"
	@echo "  lint        - Run linters"
	@echo "  clean       - Clean build artifacts"
	@echo "  run         - Build and run version command"
	@echo "  run-cmd     - Build and run a specific command"
	@echo "  release     - Build release for current platform"
	@echo "  release-all - Build release for all platforms"
	@echo "  install     - Install to \$$GOPATH/bin"
