# Build variables
BINARY_NAME=nl-to-shell
BUILD_DIR=bin
CMD_DIR=cmd/nl-to-shell

# Version information
VERSION ?= 0.1.0-dev
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags for static linking and version embedding
LDFLAGS=-w -s \
	-X 'github.com/nl-to-shell/nl-to-shell/internal/cli.Version=$(VERSION)' \
	-X 'github.com/nl-to-shell/nl-to-shell/internal/cli.GitCommit=$(GIT_COMMIT)' \
	-X 'github.com/nl-to-shell/nl-to-shell/internal/cli.BuildDate=$(BUILD_DATE)'

# CGO settings for static builds
export CGO_ENABLED=0

# Build targets
.PHONY: all build build-all build-current clean test test-coverage test-integration test-e2e deps install uninstall release help info verify

all: deps build

# Build for current platform
build: build-current

build-current:
	@echo "Building for current platform..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for all platforms using build script
build-all:
	@./scripts/build.sh all

# Individual platform builds
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./$(CMD_DIR)

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./$(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)

build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./$(CMD_DIR)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

# Test targets
test:
	$(GOTEST) -v ./...

test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-integration:
	$(GOTEST) -v -tags=integration ./...

test-e2e:
	$(GOTEST) -v -tags=e2e ./...

test-all: test test-integration test-e2e

# Dependency management
deps:
	$(GOMOD) download
	$(GOMOD) tidy

deps-update:
	$(GOMOD) get -u ./...
	$(GOMOD) tidy

# Installation targets
install: build-current
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

uninstall:
	@echo "Removing $(BINARY_NAME) from /usr/local/bin..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstallation complete"

# Release preparation
release: clean deps test build-all
	@echo "Release build complete"
	@echo "Binaries available in $(BUILD_DIR)/"
	@ls -la $(BUILD_DIR)/

# Development helpers
run: build-current
	./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

debug: 
	$(GOBUILD) -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)-debug ./$(CMD_DIR)

# Information and verification
info:
	@echo "Build Information:"
	@echo "  Version: $(VERSION)"
	@echo "  Git Commit: $(GIT_COMMIT)"
	@echo "  Build Date: $(BUILD_DATE)"
	@echo "  Binary Name: $(BINARY_NAME)"
	@echo "  Build Directory: $(BUILD_DIR)"
	@echo "  LDFLAGS: $(LDFLAGS)"

verify:
	@./scripts/build.sh verify

# Linting and formatting
fmt:
	$(GOCMD) fmt ./...

vet:
	$(GOCMD) vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

check: fmt vet lint test

# Docker targets (if needed)
docker-build:
	docker build -t $(BINARY_NAME):$(VERSION) .

docker-run: docker-build
	docker run --rm -it $(BINARY_NAME):$(VERSION) $(ARGS)

# Help target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  build         - Build the binary for current platform (alias for build-current)"
	@echo "  build-current - Build the binary for current platform"
	@echo "  build-all     - Build binaries for all supported platforms"
	@echo "  build-linux   - Build binaries for Linux (amd64, arm64)"
	@echo "  build-darwin  - Build binaries for macOS (amd64, arm64)"
	@echo "  build-windows - Build binaries for Windows (amd64)"
	@echo ""
	@echo "Test targets:"
	@echo "  test          - Run unit tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-integration - Run integration tests"
	@echo "  test-e2e      - Run end-to-end tests"
	@echo "  test-all      - Run all tests"
	@echo ""
	@echo "Development targets:"
	@echo "  run           - Build and run the binary (use ARGS=... for arguments)"
	@echo "  debug         - Build with debug symbols"
	@echo "  fmt           - Format Go code"
	@echo "  vet           - Run go vet"
	@echo "  lint          - Run golangci-lint (if installed)"
	@echo "  check         - Run fmt, vet, lint, and test"
	@echo ""
	@echo "Dependency targets:"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  deps-update   - Update dependencies to latest versions"
	@echo ""
	@echo "Installation targets:"
	@echo "  install       - Install binary to /usr/local/bin"
	@echo "  uninstall     - Remove binary from /usr/local/bin"
	@echo ""
	@echo "Release targets:"
	@echo "  release       - Prepare release build (clean, test, build-all)"
	@echo ""
	@echo "Utility targets:"
	@echo "  clean         - Clean build artifacts"
	@echo "  info          - Show build information"
	@echo "  verify        - Verify build environment"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Build and run Docker container"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION       - Set build version (default: $(VERSION))"
	@echo "  ARGS          - Arguments to pass to 'make run'"