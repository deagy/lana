.PHONY: build test lint fmt clean help

# Build the binary
build:
	@echo "Building lana..."
	go build -o lana ./cmd/lana

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

# Show help
help:
	@echo "Available targets:"
	@echo "  build      - Build the lana binary"
	@echo "  test       - Run all tests"
	@echo "  cover      - Run tests with coverage"
	@echo "  fmt        - Format code"
	@echo "  lint       - Run linters"
	@echo "  clean      - Clean build artifacts"
	@echo "  run        - Build and run version command"
	@echo "  run-cmd    - Build and run a specific command (e.g., make run-cmd CMD='config show')"
