#!/bin/bash

# Build script for nl-to-shell
# Supports cross-platform builds with version embedding

set -e

# Build configuration
BINARY_NAME="nl-to-shell"
BUILD_DIR="bin"
CMD_DIR="cmd/nl-to-shell"

# Version information
VERSION=${VERSION:-"0.1.0-dev"}
GIT_COMMIT=${GIT_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}
BUILD_DATE=${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}

# Go build flags for static linking and optimization
LDFLAGS="-w -s -X 'github.com/nl-to-shell/nl-to-shell/internal/cli.Version=${VERSION}' -X 'github.com/nl-to-shell/nl-to-shell/internal/cli.GitCommit=${GIT_COMMIT}' -X 'github.com/nl-to-shell/nl-to-shell/internal/cli.BuildDate=${BUILD_DATE}'"

# CGO settings for static builds
export CGO_ENABLED=0

# Target platforms
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to build for a specific platform
build_platform() {
    local platform=$1
    local goos=$(echo $platform | cut -d'/' -f1)
    local goarch=$(echo $platform | cut -d'/' -f2)
    
    local output_name="${BINARY_NAME}-${goos}-${goarch}"
    if [ "$goos" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    local output_path="${BUILD_DIR}/${output_name}"
    
    log_info "Building for ${goos}/${goarch}..."
    
    GOOS=$goos GOARCH=$goarch go build \
        -ldflags="$LDFLAGS" \
        -o "$output_path" \
        "./$CMD_DIR"
    
    if [ $? -eq 0 ]; then
        local size=$(du -h "$output_path" | cut -f1)
        log_success "Built ${output_name} (${size})"
        
        # Create checksum
        if command -v sha256sum >/dev/null 2>&1; then
            sha256sum "$output_path" > "${output_path}.sha256"
        elif command -v shasum >/dev/null 2>&1; then
            shasum -a 256 "$output_path" > "${output_path}.sha256"
        else
            log_warning "No checksum utility found, skipping checksum generation"
        fi
    else
        log_error "Failed to build for ${goos}/${goarch}"
        return 1
    fi
}

# Function to build for current platform only
build_current() {
    log_info "Building for current platform..."
    
    go build \
        -ldflags="$LDFLAGS" \
        -o "${BUILD_DIR}/${BINARY_NAME}" \
        "./$CMD_DIR"
    
    if [ $? -eq 0 ]; then
        local size=$(du -h "${BUILD_DIR}/${BINARY_NAME}" | cut -f1)
        log_success "Built ${BINARY_NAME} (${size})"
    else
        log_error "Failed to build for current platform"
        return 1
    fi
}

# Function to clean build artifacts
clean() {
    log_info "Cleaning build artifacts..."
    rm -rf "$BUILD_DIR"
    log_success "Build artifacts cleaned"
}

# Function to show build information
show_info() {
    echo "Build Information:"
    echo "  Version: $VERSION"
    echo "  Git Commit: $GIT_COMMIT"
    echo "  Build Date: $BUILD_DATE"
    echo "  Binary Name: $BINARY_NAME"
    echo "  Build Directory: $BUILD_DIR"
    echo ""
    echo "Supported Platforms:"
    for platform in "${PLATFORMS[@]}"; do
        echo "  - $platform"
    done
}

# Function to verify Go environment
verify_environment() {
    log_info "Verifying build environment..."
    
    # Check Go installation
    if ! command -v go >/dev/null 2>&1; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    local go_version=$(go version | cut -d' ' -f3)
    log_info "Go version: $go_version"
    
    # Check if we're in a Go module
    if [ ! -f "go.mod" ]; then
        log_error "go.mod not found. Please run from the project root directory."
        exit 1
    fi
    
    # Create build directory
    mkdir -p "$BUILD_DIR"
    
    log_success "Environment verification complete"
}

# Main function
main() {
    case "${1:-all}" in
        "current")
            verify_environment
            show_info
            build_current
            ;;
        "all")
            verify_environment
            show_info
            log_info "Building for all platforms..."
            for platform in "${PLATFORMS[@]}"; do
                build_platform "$platform"
            done
            log_success "All builds completed"
            ;;
        "clean")
            clean
            ;;
        "info")
            show_info
            ;;
        "verify")
            verify_environment
            ;;
        *)
            echo "Usage: $0 [current|all|clean|info|verify]"
            echo ""
            echo "Commands:"
            echo "  current  - Build for current platform only"
            echo "  all      - Build for all supported platforms (default)"
            echo "  clean    - Clean build artifacts"
            echo "  info     - Show build information"
            echo "  verify   - Verify build environment"
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"