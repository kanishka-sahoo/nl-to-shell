#!/bin/bash

# Installation script for nl-to-shell
# Supports Linux and macOS

set -e

# Configuration
BINARY_NAME="nl-to-shell"
REPO_OWNER="nl-to-shell"
REPO_NAME="nl-to-shell"
INSTALL_DIR="/usr/local/bin"
TMP_DIR="/tmp/nl-to-shell-install"

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

# Function to detect OS and architecture
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case $os in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
    
    case $arch in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
    
    PLATFORM="${OS}-${ARCH}"
    log_info "Detected platform: $PLATFORM"
}

# Function to get the latest release version
get_latest_version() {
    log_info "Fetching latest release information..."
    
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -s "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        log_error "Neither curl nor wget is available. Please install one of them."
        exit 1
    fi
    
    if [ -z "$VERSION" ]; then
        log_error "Failed to fetch latest version"
        exit 1
    fi
    
    log_info "Latest version: $VERSION"
}

# Function to download and verify binary
download_binary() {
    local binary_name="${BINARY_NAME}-${PLATFORM}"
    local archive_name="${binary_name}.tar.gz"
    local download_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/${archive_name}"
    local checksum_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/${binary_name}.sha256"
    
    log_info "Downloading $archive_name..."
    
    # Create temporary directory
    mkdir -p "$TMP_DIR"
    cd "$TMP_DIR"
    
    # Download archive
    if command -v curl >/dev/null 2>&1; then
        curl -L -o "$archive_name" "$download_url"
        curl -L -o "${binary_name}.sha256" "$checksum_url"
    elif command -v wget >/dev/null 2>&1; then
        wget -O "$archive_name" "$download_url"
        wget -O "${binary_name}.sha256" "$checksum_url"
    fi
    
    # Verify download
    if [ ! -f "$archive_name" ]; then
        log_error "Failed to download $archive_name"
        exit 1
    fi
    
    log_success "Downloaded $archive_name"
    
    # Extract archive
    log_info "Extracting archive..."
    tar -xzf "$archive_name"
    
    # Verify checksum if available
    if [ -f "${binary_name}.sha256" ]; then
        log_info "Verifying checksum..."
        if command -v sha256sum >/dev/null 2>&1; then
            if sha256sum -c "${binary_name}.sha256"; then
                log_success "Checksum verification passed"
            else
                log_error "Checksum verification failed"
                exit 1
            fi
        elif command -v shasum >/dev/null 2>&1; then
            if shasum -a 256 -c "${binary_name}.sha256"; then
                log_success "Checksum verification passed"
            else
                log_error "Checksum verification failed"
                exit 1
            fi
        else
            log_warning "No checksum utility found, skipping verification"
        fi
    else
        log_warning "Checksum file not found, skipping verification"
    fi
    
    # Make binary executable
    chmod +x "$binary_name"
}

# Function to install binary
install_binary() {
    local binary_name="${BINARY_NAME}-${PLATFORM}"
    local source_path="${TMP_DIR}/${binary_name}"
    local target_path="${INSTALL_DIR}/${BINARY_NAME}"
    
    log_info "Installing $BINARY_NAME to $INSTALL_DIR..."
    
    # Check if install directory exists and is writable
    if [ ! -d "$INSTALL_DIR" ]; then
        log_error "Install directory $INSTALL_DIR does not exist"
        exit 1
    fi
    
    # Check if we need sudo
    if [ ! -w "$INSTALL_DIR" ]; then
        log_info "Installing with sudo (requires administrator privileges)..."
        sudo cp "$source_path" "$target_path"
        sudo chmod +x "$target_path"
    else
        cp "$source_path" "$target_path"
        chmod +x "$target_path"
    fi
    
    log_success "Installed $BINARY_NAME to $target_path"
}

# Function to verify installation
verify_installation() {
    log_info "Verifying installation..."
    
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        local installed_version=$($BINARY_NAME version | grep -o 'version [^ ]*' | cut -d' ' -f2)
        log_success "$BINARY_NAME $installed_version installed successfully"
        
        # Show usage information
        echo ""
        echo "Usage:"
        echo "  $BINARY_NAME \"your natural language command\""
        echo "  $BINARY_NAME --help"
        echo ""
        echo "Examples:"
        echo "  $BINARY_NAME \"list files by size\""
        echo "  $BINARY_NAME \"find large files in current directory\""
        echo ""
    else
        log_error "Installation verification failed. $BINARY_NAME not found in PATH."
        log_info "You may need to add $INSTALL_DIR to your PATH or restart your shell."
        exit 1
    fi
}

# Function to cleanup temporary files
cleanup() {
    log_info "Cleaning up temporary files..."
    rm -rf "$TMP_DIR"
}

# Function to show help
show_help() {
    echo "nl-to-shell installer"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -v, --version VERSION    Install specific version (default: latest)"
    echo "  -d, --dir DIRECTORY      Install directory (default: $INSTALL_DIR)"
    echo "  -h, --help               Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                       Install latest version"
    echo "  $0 --version v1.0.0      Install specific version"
    echo "  $0 --dir ~/.local/bin    Install to custom directory"
}

# Function to check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check for required tools
    local missing_tools=()
    
    if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
        missing_tools+=("curl or wget")
    fi
    
    if ! command -v tar >/dev/null 2>&1; then
        missing_tools+=("tar")
    fi
    
    if [ ${#missing_tools[@]} -gt 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        echo "Please install the missing tools and try again."
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Main installation function
main() {
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -v|--version)
                VERSION="$2"
                shift 2
                ;;
            -d|--dir)
                INSTALL_DIR="$2"
                shift 2
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    echo "nl-to-shell installer"
    echo "===================="
    echo ""
    
    # Run installation steps
    check_prerequisites
    detect_platform
    
    if [ -z "$VERSION" ]; then
        get_latest_version
    else
        log_info "Using specified version: $VERSION"
    fi
    
    download_binary
    install_binary
    verify_installation
    cleanup
    
    echo ""
    log_success "Installation completed successfully!"
    echo ""
    echo "Get started with:"
    echo "  $BINARY_NAME --help"
}

# Set up trap for cleanup on exit
trap cleanup EXIT

# Run main function with all arguments
main "$@"