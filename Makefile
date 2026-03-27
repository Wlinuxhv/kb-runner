.PHONY: all clean build build-linux build-windows build-darwin test help embed-kbscripts

BINARY_NAME=kb-runner
VERSION:=$(shell git describe --tags --always --dirty)
BUILD_TIME:=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Embedded KB scripts directory
EMBED_KB_DIR := internal/kbscripts
KB_SCRIPTS_DIR := kbscript

# Default target
all: embed-kbscripts build

# Build for current platform
build: embed-kbscripts build-linux

# Build for Linux
build-linux:
	@echo "Building Linux binary..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/kb-runner
	@echo "Linux binary built: bin/$(BINARY_NAME)"

# Build for Windows
build-windows:
	@echo "Building Windows binary..."
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME).exe ./cmd/kb-runner
	@echo "Windows binary built: bin/$(BINARY_NAME).exe"

# Build for Darwin (macOS)
build-darwin:
	@echo "Building macOS binary..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin ./cmd/kb-runner
	@echo "macOS binary built: bin/$(BINARY_NAME)-darwin"

# Build all platforms
build-all: build-linux build-windows build-darwin
	@echo "All binaries built successfully!"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf release/
	@echo "Cleaned."

# Create release package
release: build-all
	@echo "Creating release package..."
	mkdir -p release/linux
	mkdir -p release/windows
	mkdir -p release/configs
	mkdir -p release/scripts
	cp bin/kb-runner release/linux/
	cp bin/kb-runner.exe release/windows/
	cp -r configs release/
	cp -r scripts release/
	@echo "Release package created in release/"

# Embed KB scripts
embed-kbscripts:
	@echo "Embedding KB scripts..."
	@mkdir -p $(EMBED_KB_DIR)
	@cp -r $(KB_SCRIPTS_DIR)/* $(EMBED_KB_DIR)/ 2>/dev/null || true
	@echo "KB scripts embedded successfully to $(EMBED_KB_DIR)"

# Help
help:
	@echo "KB Runner Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build         Build for current platform (default: Linux)"
	@echo "  build-linux   Build Linux amd64 binary"
	@echo "  build-windows Build Windows amd64 binary"
	@echo "  build-darwin Build macOS amd64 binary"
	@echo "  build-all    Build all platform binaries"
	@echo "  release      Create release package with all binaries"
	@echo "  test         Run tests"
	@echo "  clean        Clean build artifacts"
	@echo "  embed-kbscripts  Embed KB scripts into binary"
	@echo "  help         Show this help message"
