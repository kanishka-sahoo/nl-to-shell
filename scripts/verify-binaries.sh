#!/bin/bash

# Binary verification script for nl-to-shell
# Verifies checksums and optionally signs binaries

set -e

# Configuration
BINARY_NAME="nl-to-shell"
BUILD_DIR="bin"
SIGNING_KEY=${SIGNING_KEY:-""}
GPG_KEY_ID=${GPG_KEY_ID:-""}

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

# Function to verify checksums
verify_checksums() {
    log_info "Verifying binary checksums..."
    
    local failed=0
    
    for checksum_file in "$BUILD_DIR"/*.sha256; do
        if [ -f "$checksum_file" ]; then
            local binary_name=$(basename "$checksum_file" .sha256)
            local binary_path="$BUILD_DIR/$binary_name"
            
            if [ -f "$binary_path" ]; then
                log_info "Verifying $binary_name..."
                
                if command -v sha256sum >/dev/null 2>&1; then
                    if sha256sum -c "$checksum_file"; then
                        log_success "✓ $binary_name checksum verified"
                    else
                        log_error "✗ $binary_name checksum verification failed"
                        failed=1
                    fi
                elif command -v shasum >/dev/null 2>&1; then
                    if shasum -a 256 -c "$checksum_file"; then
                        log_success "✓ $binary_name checksum verified"
                    else
                        log_error "✗ $binary_name checksum verification failed"
                        failed=1
                    fi
                else
                    log_warning "No checksum utility found, skipping verification for $binary_name"
                fi
            else
                log_warning "Binary $binary_path not found, skipping"
            fi
        fi
    done
    
    if [ $failed -eq 0 ]; then
        log_success "All checksums verified successfully"
        return 0
    else
        log_error "Some checksum verifications failed"
        return 1
    fi
}

# Function to sign binaries with GPG
sign_binaries_gpg() {
    log_info "Signing binaries with GPG..."
    
    if [ -z "$GPG_KEY_ID" ]; then
        log_error "GPG_KEY_ID environment variable not set"
        return 1
    fi
    
    if ! command -v gpg >/dev/null 2>&1; then
        log_error "GPG not found. Please install GPG to sign binaries."
        return 1
    fi
    
    # Check if key exists
    if ! gpg --list-secret-keys "$GPG_KEY_ID" >/dev/null 2>&1; then
        log_error "GPG key $GPG_KEY_ID not found in keyring"
        return 1
    fi
    
    local signed_count=0
    
    for binary in "$BUILD_DIR"/$BINARY_NAME-*; do
        if [ -f "$binary" ] && [ ! -f "$binary.sig" ]; then
            log_info "Signing $(basename "$binary")..."
            
            if gpg --detach-sign --armor --local-user "$GPG_KEY_ID" "$binary"; then
                log_success "✓ Signed $(basename "$binary")"
                signed_count=$((signed_count + 1))
            else
                log_error "✗ Failed to sign $(basename "$binary")"
                return 1
            fi
        fi
    done
    
    log_success "Signed $signed_count binaries with GPG"
}

# Function to verify GPG signatures
verify_signatures() {
    log_info "Verifying GPG signatures..."
    
    if ! command -v gpg >/dev/null 2>&1; then
        log_warning "GPG not found. Skipping signature verification."
        return 0
    fi
    
    local verified_count=0
    local failed=0
    
    for sig_file in "$BUILD_DIR"/*.sig; do
        if [ -f "$sig_file" ]; then
            local binary_file="${sig_file%.sig}"
            
            if [ -f "$binary_file" ]; then
                log_info "Verifying signature for $(basename "$binary_file")..."
                
                if gpg --verify "$sig_file" "$binary_file" 2>/dev/null; then
                    log_success "✓ Signature verified for $(basename "$binary_file")"
                    verified_count=$((verified_count + 1))
                else
                    log_error "✗ Signature verification failed for $(basename "$binary_file")"
                    failed=1
                fi
            fi
        fi
    done
    
    if [ $verified_count -gt 0 ]; then
        if [ $failed -eq 0 ]; then
            log_success "All $verified_count signatures verified successfully"
            return 0
        else
            log_error "Some signature verifications failed"
            return 1
        fi
    else
        log_info "No signatures found to verify"
        return 0
    fi
}

# Function to create SLSA provenance (basic implementation)
create_provenance() {
    log_info "Creating SLSA provenance information..."
    
    local provenance_file="$BUILD_DIR/provenance.json"
    local build_date=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local git_commit=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
    local git_repo=$(git config --get remote.origin.url 2>/dev/null || echo "unknown")
    
    cat > "$provenance_file" << EOF
{
  "_type": "https://in-toto.io/Statement/v0.1",
  "predicateType": "https://slsa.dev/provenance/v0.2",
  "subject": [
$(
    first=true
    for binary in "$BUILD_DIR"/$BINARY_NAME-*; do
        if [ -f "$binary" ] && [ ! "${binary##*.}" = "sha256" ] && [ ! "${binary##*.}" = "sig" ]; then
            if [ "$first" = true ]; then
                first=false
            else
                echo ","
            fi
            local sha256=""
            if [ -f "$binary.sha256" ]; then
                sha256=$(cut -d' ' -f1 "$binary.sha256")
            fi
            echo "    {"
            echo "      \"name\": \"$(basename "$binary")\","
            echo "      \"digest\": {"
            echo "        \"sha256\": \"$sha256\""
            echo "      }"
            echo -n "    }"
        fi
    done
)
  ],
  "predicate": {
    "builder": {
      "id": "https://github.com/nl-to-shell/nl-to-shell/.github/workflows/build.yml"
    },
    "buildType": "https://github.com/Attestations/GitHubActionsWorkflow@v1",
    "invocation": {
      "configSource": {
        "uri": "$git_repo",
        "digest": {
          "sha1": "$git_commit"
        }
      }
    },
    "metadata": {
      "buildInvocationId": "${GITHUB_RUN_ID:-local}",
      "buildStartedOn": "$build_date",
      "completeness": {
        "parameters": true,
        "environment": false,
        "materials": false
      },
      "reproducible": false
    },
    "materials": [
      {
        "uri": "$git_repo",
        "digest": {
          "sha1": "$git_commit"
        }
      }
    ]
  }
}
EOF
    
    log_success "Created provenance file: $provenance_file"
}

# Function to generate verification report
generate_report() {
    log_info "Generating verification report..."
    
    local report_file="$BUILD_DIR/verification-report.txt"
    local timestamp=$(date -u +"%Y-%m-%d %H:%M:%S UTC")
    
    cat > "$report_file" << EOF
nl-to-shell Binary Verification Report
=====================================

Generated: $timestamp
Build Directory: $BUILD_DIR

Binaries Found:
EOF
    
    for binary in "$BUILD_DIR"/$BINARY_NAME-*; do
        if [ -f "$binary" ] && [ ! "${binary##*.}" = "sha256" ] && [ ! "${binary##*.}" = "sig" ]; then
            local size=$(du -h "$binary" | cut -f1)
            local checksum=""
            local signature_status="No signature"
            
            if [ -f "$binary.sha256" ]; then
                checksum=$(cut -d' ' -f1 "$binary.sha256")
            fi
            
            if [ -f "$binary.sig" ]; then
                if gpg --verify "$binary.sig" "$binary" 2>/dev/null; then
                    signature_status="Valid signature"
                else
                    signature_status="Invalid signature"
                fi
            fi
            
            cat >> "$report_file" << EOF

- $(basename "$binary")
  Size: $size
  SHA256: $checksum
  Signature: $signature_status
EOF
        fi
    done
    
    cat >> "$report_file" << EOF

Verification Tools:
- SHA256: $(command -v sha256sum >/dev/null 2>&1 && echo "sha256sum available" || (command -v shasum >/dev/null 2>&1 && echo "shasum available" || echo "Not available"))
- GPG: $(command -v gpg >/dev/null 2>&1 && echo "$(gpg --version | head -n1)" || echo "Not available")

Environment:
- OS: $(uname -s)
- Architecture: $(uname -m)
- User: $(whoami)
- Working Directory: $(pwd)
EOF
    
    log_success "Generated verification report: $report_file"
}

# Function to show help
show_help() {
    echo "Binary verification script for nl-to-shell"
    echo ""
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  verify      Verify binary checksums (default)"
    echo "  sign        Sign binaries with GPG"
    echo "  verify-sigs Verify GPG signatures"
    echo "  provenance  Create SLSA provenance information"
    echo "  report      Generate verification report"
    echo "  all         Run all verification and signing steps"
    echo "  help        Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  GPG_KEY_ID  GPG key ID for signing (required for signing)"
    echo ""
    echo "Examples:"
    echo "  $0 verify                    # Verify checksums only"
    echo "  GPG_KEY_ID=ABC123 $0 sign    # Sign binaries with GPG key"
    echo "  $0 all                       # Run all steps"
}

# Main function
main() {
    if [ ! -d "$BUILD_DIR" ]; then
        log_error "Build directory $BUILD_DIR not found. Run build first."
        exit 1
    fi
    
    case "${1:-verify}" in
        "verify")
            verify_checksums
            ;;
        "sign")
            sign_binaries_gpg
            ;;
        "verify-sigs")
            verify_signatures
            ;;
        "provenance")
            create_provenance
            ;;
        "report")
            generate_report
            ;;
        "all")
            verify_checksums
            if [ -n "$GPG_KEY_ID" ]; then
                sign_binaries_gpg
                verify_signatures
            else
                log_info "GPG_KEY_ID not set, skipping signing"
            fi
            create_provenance
            generate_report
            log_success "All verification steps completed"
            ;;
        "help"|"-h"|"--help")
            show_help
            ;;
        *)
            log_error "Unknown command: $1"
            show_help
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"