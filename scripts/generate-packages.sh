#!/bin/bash

# Script to generate package manager configurations
# Supports Homebrew, APT, and Chocolatey

set -e

# Configuration
BINARY_NAME="nl-to-shell"
REPO_OWNER="nl-to-shell"
REPO_NAME="nl-to-shell"
VERSION=${VERSION:-"0.1.0-dev"}
DESCRIPTION="Convert natural language to shell commands using LLMs"
HOMEPAGE="https://github.com/${REPO_OWNER}/${REPO_NAME}"

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

# Function to generate Homebrew formula
generate_homebrew_formula() {
    log_info "Generating Homebrew formula..."
    
    local formula_dir="packaging/homebrew"
    local formula_file="${formula_dir}/nl-to-shell.rb"
    
    mkdir -p "$formula_dir"
    
    # Get checksums for Darwin binaries (if they exist)
    local darwin_amd64_sha256=""
    local darwin_arm64_sha256=""
    
    if [ -f "bin/nl-to-shell-darwin-amd64.sha256" ]; then
        darwin_amd64_sha256=$(cut -d' ' -f1 bin/nl-to-shell-darwin-amd64.sha256)
    fi
    
    if [ -f "bin/nl-to-shell-darwin-arm64.sha256" ]; then
        darwin_arm64_sha256=$(cut -d' ' -f1 bin/nl-to-shell-darwin-arm64.sha256)
    fi
    
    cat > "$formula_file" << EOF
class NlToShell < Formula
  desc "$DESCRIPTION"
  homepage "$HOMEPAGE"
  url "${HOMEPAGE}/releases/download/${VERSION}/nl-to-shell-darwin-amd64.tar.gz"
  sha256 "$darwin_amd64_sha256"
  license "MIT"
  version "$VERSION"

  depends_on "git"

  on_arm do
    url "${HOMEPAGE}/releases/download/${VERSION}/nl-to-shell-darwin-arm64.tar.gz"
    sha256 "$darwin_arm64_sha256"
  end

  def install
    bin.install "nl-to-shell-darwin-amd64" => "nl-to-shell" if Hardware::CPU.intel?
    bin.install "nl-to-shell-darwin-arm64" => "nl-to-shell" if Hardware::CPU.arm?
  end

  test do
    system "#{bin}/nl-to-shell", "version"
  end
end
EOF
    
    log_success "Generated Homebrew formula: $formula_file"
}

# Function to generate Debian package control files
generate_debian_package() {
    log_info "Generating Debian package configuration..."
    
    local deb_dir="packaging/debian"
    local control_file="${deb_dir}/DEBIAN/control"
    
    mkdir -p "${deb_dir}/DEBIAN"
    mkdir -p "${deb_dir}/usr/local/bin"
    
    # Generate control file
    cat > "$control_file" << EOF
Package: nl-to-shell
Version: ${VERSION#v}
Section: utils
Priority: optional
Architecture: amd64
Depends: git
Maintainer: nl-to-shell Team <maintainers@nl-to-shell.com>
Description: $DESCRIPTION
 nl-to-shell is a CLI utility that converts natural language descriptions
 into executable shell commands using Large Language Models (LLMs).
 .
 It provides context-aware command generation by analyzing your current working
 directory, git repository state, files, and other environmental factors.
Homepage: $HOMEPAGE
EOF
    
    # Generate postinst script
    cat > "${deb_dir}/DEBIAN/postinst" << 'EOF'
#!/bin/bash
set -e

# Make binary executable
chmod +x /usr/local/bin/nl-to-shell

# Update PATH if needed
if ! echo "$PATH" | grep -q "/usr/local/bin"; then
    echo "Note: /usr/local/bin should be in your PATH to use nl-to-shell"
fi

echo "nl-to-shell installed successfully!"
echo "Run 'nl-to-shell --help' to get started."
EOF
    
    chmod +x "${deb_dir}/DEBIAN/postinst"
    
    # Generate prerm script
    cat > "${deb_dir}/DEBIAN/prerm" << 'EOF'
#!/bin/bash
set -e

echo "Removing nl-to-shell..."
EOF
    
    chmod +x "${deb_dir}/DEBIAN/prerm"
    
    log_success "Generated Debian package configuration: $deb_dir"
}

# Function to generate Chocolatey package
generate_chocolatey_package() {
    log_info "Generating Chocolatey package configuration..."
    
    local choco_dir="packaging/chocolatey"
    local nuspec_file="${choco_dir}/nl-to-shell.nuspec"
    local install_script="${choco_dir}/tools/chocolateyinstall.ps1"
    
    mkdir -p "${choco_dir}/tools"
    
    # Generate nuspec file
    cat > "$nuspec_file" << EOF
<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2015/06/nuspec.xsd">
  <metadata>
    <id>nl-to-shell</id>
    <version>${VERSION#v}</version>
    <packageSourceUrl>$HOMEPAGE</packageSourceUrl>
    <owners>nl-to-shell</owners>
    <title>nl-to-shell</title>
    <authors>nl-to-shell Team</authors>
    <projectUrl>$HOMEPAGE</projectUrl>
    <iconUrl>$HOMEPAGE/raw/main/assets/icon.png</iconUrl>
    <copyright>2025 nl-to-shell Team</copyright>
    <licenseUrl>$HOMEPAGE/blob/main/LICENSE</licenseUrl>
    <requireLicenseAcceptance>false</requireLicenseAcceptance>
    <projectSourceUrl>$HOMEPAGE</projectSourceUrl>
    <docsUrl>$HOMEPAGE/blob/main/README.md</docsUrl>
    <bugTrackerUrl>$HOMEPAGE/issues</bugTrackerUrl>
    <tags>cli shell command natural-language llm ai</tags>
    <summary>$DESCRIPTION</summary>
    <description>
nl-to-shell is a CLI utility that converts natural language descriptions into executable shell commands using Large Language Models (LLMs).

## Features

- Context-aware command generation
- Multiple AI provider support (OpenAI, Anthropic, Google, etc.)
- Safety validation and confirmation prompts
- Cross-platform compatibility
- Interactive session mode
- Result validation and correction

## Usage

\`\`\`
nl-to-shell "list files by size in descending order"
nl-to-shell --dry-run "delete all .tmp files"
nl-to-shell --provider openai "find large files"
\`\`\`
    </description>
    <releaseNotes>$HOMEPAGE/releases/tag/$VERSION</releaseNotes>
  </metadata>
  <files>
    <file src="tools\**" target="tools" />
  </files>
</package>
EOF
    
    # Generate install script
    cat > "$install_script" << EOF
\$ErrorActionPreference = 'Stop'

\$packageName = 'nl-to-shell'
\$toolsDir = "\$(Split-Path -parent \$MyInvocation.MyCommand.Definition)"
\$url64 = '${HOMEPAGE}/releases/download/${VERSION}/nl-to-shell-windows-amd64.exe'

\$packageArgs = @{
  packageName   = \$packageName
  unzipLocation = \$toolsDir
  fileType      = 'exe'
  url64bit      = \$url64
  softwareName  = 'nl-to-shell*'
  checksum64    = 'PLACEHOLDER_CHECKSUM'
  checksumType64= 'sha256'
  silentArgs    = '/S'
  validExitCodes= @(0)
}

Install-ChocolateyPackage @packageArgs
EOF
    
    # Generate uninstall script
    cat > "${choco_dir}/tools/chocolateyuninstall.ps1" << 'EOF'
$ErrorActionPreference = 'Stop'

$packageName = 'nl-to-shell'
$softwareName = 'nl-to-shell*'

[array]$key = Get-UninstallRegistryKey -SoftwareName $softwareName

if ($key.Count -eq 1) {
  $key | % { 
    $packageArgs = @{
      packageName = $packageName
      fileType    = 'exe'
      silentArgs  = '/S'
      validExitCodes= @(0)
      file        = "$($_.UninstallString)"
    }
    
    Uninstall-ChocolateyPackage @packageArgs
  }
} elseif ($key.Count -eq 0) {
  Write-Warning "$packageName has already been uninstalled by other means."
} elseif ($key.Count -gt 1) {
  Write-Warning "$key.Count matches found!"
  Write-Warning "To prevent accidental data loss, no programs will be uninstalled."
  Write-Warning "Please alert package maintainer the following keys were matched:"
  $key | % {Write-Warning "- $_.DisplayName"}
}
EOF
    
    log_success "Generated Chocolatey package configuration: $choco_dir"
}

# Function to generate RPM spec file
generate_rpm_spec() {
    log_info "Generating RPM spec file..."
    
    local rpm_dir="packaging/rpm"
    local spec_file="${rpm_dir}/nl-to-shell.spec"
    
    mkdir -p "$rpm_dir"
    
    cat > "$spec_file" << EOF
Name:           nl-to-shell
Version:        ${VERSION#v}
Release:        1%{?dist}
Summary:        $DESCRIPTION

License:        MIT
URL:            $HOMEPAGE
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  git
Requires:       git

%description
nl-to-shell is a CLI utility that converts natural language descriptions
into executable shell commands using Large Language Models (LLMs).

It provides context-aware command generation by analyzing your current working
directory, git repository state, files, and other environmental factors.

%prep
%setup -q

%build
# Binary is pre-built

%install
rm -rf \$RPM_BUILD_ROOT
mkdir -p \$RPM_BUILD_ROOT%{_bindir}
install -m 755 nl-to-shell-linux-amd64 \$RPM_BUILD_ROOT%{_bindir}/nl-to-shell

%clean
rm -rf \$RPM_BUILD_ROOT

%files
%defattr(-,root,root,-)
%{_bindir}/nl-to-shell

%changelog
* $(date +'%a %b %d %Y') nl-to-shell Team <maintainers@nl-to-shell.com> - ${VERSION#v}-1
- Initial package release
EOF
    
    log_success "Generated RPM spec file: $spec_file"
}

# Function to generate all packages
generate_all() {
    log_info "Generating all package configurations..."
    
    generate_homebrew_formula
    generate_debian_package
    generate_chocolatey_package
    generate_rpm_spec
    
    log_success "All package configurations generated successfully!"
}

# Function to show help
show_help() {
    echo "Package generator for nl-to-shell"
    echo ""
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  homebrew    Generate Homebrew formula"
    echo "  debian      Generate Debian package configuration"
    echo "  chocolatey  Generate Chocolatey package configuration"
    echo "  rpm         Generate RPM spec file"
    echo "  all         Generate all package configurations (default)"
    echo "  help        Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  VERSION     Package version (default: $VERSION)"
}

# Main function
main() {
    case "${1:-all}" in
        "homebrew")
            generate_homebrew_formula
            ;;
        "debian")
            generate_debian_package
            ;;
        "chocolatey")
            generate_chocolatey_package
            ;;
        "rpm")
            generate_rpm_spec
            ;;
        "all")
            generate_all
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