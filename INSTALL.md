# Installation Guide

This document provides various methods to install nl-to-shell on your system.

## Quick Install

### Linux and macOS

```bash
curl -sSL https://github.com/kanishka-sahoo/nl-to-shell/releases/latest/download/install.sh | bash
```

### Windows (PowerShell)

```powershell
iwr https://github.com/kanishka-sahoo/nl-to-shell/releases/latest/download/install.ps1 | iex
```

## Package Managers

### Homebrew (macOS/Linux)

```bash
# Add the tap
brew tap nl-to-shell/tap

# Install nl-to-shell
brew install nl-to-shell
```

### Chocolatey (Windows)

```powershell
choco install nl-to-shell
```

### APT (Debian/Ubuntu)

```bash
# Download and install the .deb package
wget https://github.com/kanishka-sahoo/nl-to-shell/releases/latest/download/nl-to-shell-linux-amd64.deb
sudo dpkg -i nl-to-shell-linux-amd64.deb
```

### RPM (RHEL/CentOS/Fedora)

```bash
# Download and install the .rpm package
wget https://github.com/kanishka-sahoo/nl-to-shell/releases/latest/download/nl-to-shell-linux-amd64.rpm
sudo rpm -i nl-to-shell-linux-amd64.rpm
```

## Docker

### Run directly

```bash
docker run --rm -it ghcr.io/kanishka-sahoo/nl-to-shell:latest "list files by size"
```

### With volume mount for current directory

```bash
docker run --rm -it -v "$(pwd):/workspace" -w /workspace ghcr.io/kanishka-sahoo/nl-to-shell:latest "list files by size"
```

## Manual Installation

### Download Pre-built Binaries

1. Go to the [releases page](https://github.com/kanishka-sahoo/nl-to-shell/releases)
2. Download the appropriate binary for your platform:
   - `nl-to-shell-linux-amd64` - Linux x86_64
   - `nl-to-shell-linux-arm64` - Linux ARM64
   - `nl-to-shell-darwin-amd64` - macOS Intel
   - `nl-to-shell-darwin-arm64` - macOS Apple Silicon
   - `nl-to-shell-windows-amd64.exe` - Windows x86_64

3. Make the binary executable (Linux/macOS):
   ```bash
   chmod +x nl-to-shell-*
   ```

4. Move to a directory in your PATH:
   ```bash
   # Linux/macOS
   sudo mv nl-to-shell-* /usr/local/bin/nl-to-shell
   
   # Windows - move to a directory in your PATH or add the directory to PATH
   ```

### Verify Installation

```bash
nl-to-shell version
```

### Build from Source

Requirements:
- Go 1.23 or later
- Git

```bash
# Clone the repository
git clone https://github.com/kanishka-sahoo/nl-to-shell.git
cd nl-to-shell

# Build for your platform
make build

# Or build for all platforms
make build-all

# Install to /usr/local/bin (Linux/macOS)
sudo make install
```

## Configuration

After installation, you'll need to configure your AI provider credentials:

```bash
# Interactive setup
nl-to-shell config setup

# Or set environment variables
export OPENAI_API_KEY="your-api-key"
export ANTHROPIC_API_KEY="your-api-key"
# etc.
```

## Verification

### Verify Binary Integrity

All releases include SHA256 checksums. To verify:

```bash
# Download the binary and checksum
wget https://github.com/kanishka-sahoo/nl-to-shell/releases/latest/download/nl-to-shell-linux-amd64
wget https://github.com/kanishka-sahoo/nl-to-shell/releases/latest/download/nl-to-shell-linux-amd64.sha256

# Verify checksum
sha256sum -c nl-to-shell-linux-amd64.sha256
```

### GPG Signatures

Releases are signed with GPG. To verify signatures:

```bash
# Import the signing key
gpg --keyserver keyserver.ubuntu.com --recv-keys [KEY_ID]

# Verify signature
gpg --verify nl-to-shell-linux-amd64.sig nl-to-shell-linux-amd64
```

## Troubleshooting

### Permission Denied

If you get permission denied errors:

```bash
# Make sure the binary is executable
chmod +x /path/to/nl-to-shell

# Check if the directory is in your PATH
echo $PATH
```

### Command Not Found

If the command is not found after installation:

1. Restart your shell or open a new terminal
2. Check if the installation directory is in your PATH
3. Try running with the full path: `/usr/local/bin/nl-to-shell`

### API Key Issues

If you get API key errors:

1. Make sure you've configured your provider credentials
2. Check that the API key is valid and has sufficient credits
3. Verify the provider name matches the supported providers

## Uninstallation

### Package Managers

```bash
# Homebrew
brew uninstall nl-to-shell

# Chocolatey
choco uninstall nl-to-shell

# APT
sudo apt remove nl-to-shell

# RPM
sudo rpm -e nl-to-shell
```

### Manual Installation

```bash
# Remove the binary
sudo rm /usr/local/bin/nl-to-shell

# Remove configuration (optional)
rm -rf ~/.config/nl-to-shell
```

## Getting Help

- [Documentation](https://github.com/kanishka-sahoo/nl-to-shell/blob/main/README.md)
- [Issues](https://github.com/kanishka-sahoo/nl-to-shell/issues)
- [Discussions](https://github.com/kanishka-sahoo/nl-to-shell/discussions)

For installation-specific issues, please include:
- Your operating system and version
- Installation method used
- Complete error messages
- Output of `nl-to-shell version` (if the binary runs)