# PowerShell installation script for nl-to-shell
# Supports Windows

param(
    [Parameter()]
    [string]$Version = "",
    
    [Parameter()]
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\nl-to-shell",
    
    [Parameter()]
    [switch]$Help
)

# Configuration
$BINARY_NAME = "nl-to-shell"
$REPO_OWNER = "nl-to-shell"
$REPO_NAME = "nl-to-shell"
$TMP_DIR = "$env:TEMP\nl-to-shell-install"

# Helper functions
function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Blue
}

function Write-Success {
    param([string]$Message)
    Write-Host "[SUCCESS] $Message" -ForegroundColor Green
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[WARNING] $Message" -ForegroundColor Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

# Function to detect platform
function Get-Platform {
    $os = "windows"
    $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
    
    $platform = "$os-$arch"
    Write-Info "Detected platform: $platform"
    return $platform
}

# Function to get the latest release version
function Get-LatestVersion {
    Write-Info "Fetching latest release information..."
    
    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest"
        $version = $response.tag_name
        
        if (-not $version) {
            throw "No version found in response"
        }
        
        Write-Info "Latest version: $version"
        return $version
    }
    catch {
        Write-Error "Failed to fetch latest version: $($_.Exception.Message)"
        exit 1
    }
}

# Function to download and verify binary
function Download-Binary {
    param(
        [string]$Version,
        [string]$Platform
    )
    
    $binaryName = "$BINARY_NAME-$Platform.exe"
    $archiveName = "$BINARY_NAME-$Platform.zip"
    $downloadUrl = "https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/$Version/$archiveName"
    $checksumUrl = "https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/$Version/$binaryName.sha256"
    
    Write-Info "Downloading $archiveName..."
    
    # Create temporary directory
    if (Test-Path $TMP_DIR) {
        Remove-Item -Recurse -Force $TMP_DIR
    }
    New-Item -ItemType Directory -Path $TMP_DIR | Out-Null
    
    $archivePath = Join-Path $TMP_DIR $archiveName
    $checksumPath = Join-Path $TMP_DIR "$binaryName.sha256"
    
    try {
        # Download archive
        Invoke-WebRequest -Uri $downloadUrl -OutFile $archivePath
        Write-Success "Downloaded $archiveName"
        
        # Download checksum
        try {
            Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumPath
        }
        catch {
            Write-Warning "Could not download checksum file, skipping verification"
        }
        
        # Extract archive
        Write-Info "Extracting archive..."
        Expand-Archive -Path $archivePath -DestinationPath $TMP_DIR -Force
        
        # Verify checksum if available
        if (Test-Path $checksumPath) {
            Write-Info "Verifying checksum..."
            $expectedHash = (Get-Content $checksumPath).Split()[0].ToUpper()
            $actualHash = (Get-FileHash -Path (Join-Path $TMP_DIR $binaryName) -Algorithm SHA256).Hash
            
            if ($expectedHash -eq $actualHash) {
                Write-Success "Checksum verification passed"
            }
            else {
                Write-Error "Checksum verification failed"
                Write-Error "Expected: $expectedHash"
                Write-Error "Actual: $actualHash"
                exit 1
            }
        }
        else {
            Write-Warning "Checksum file not found, skipping verification"
        }
    }
    catch {
        Write-Error "Failed to download binary: $($_.Exception.Message)"
        exit 1
    }
    
    return (Join-Path $TMP_DIR $binaryName)
}

# Function to install binary
function Install-Binary {
    param(
        [string]$SourcePath,
        [string]$InstallDir
    )
    
    Write-Info "Installing $BINARY_NAME to $InstallDir..."
    
    # Create install directory
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    $targetPath = Join-Path $InstallDir "$BINARY_NAME.exe"
    
    try {
        Copy-Item -Path $SourcePath -Destination $targetPath -Force
        Write-Success "Installed $BINARY_NAME to $targetPath"
        return $targetPath
    }
    catch {
        Write-Error "Failed to install binary: $($_.Exception.Message)"
        exit 1
    }
}

# Function to add to PATH
function Add-ToPath {
    param([string]$InstallDir)
    
    Write-Info "Adding $InstallDir to PATH..."
    
    # Get current user PATH
    $currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    
    # Check if already in PATH
    if ($currentPath -split ";" | Where-Object { $_ -eq $InstallDir }) {
        Write-Info "Directory already in PATH"
        return
    }
    
    # Add to PATH
    $newPath = if ($currentPath) { "$currentPath;$InstallDir" } else { $InstallDir }
    
    try {
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        Write-Success "Added to PATH (restart shell to take effect)"
        
        # Also add to current session
        $env:PATH = "$env:PATH;$InstallDir"
    }
    catch {
        Write-Warning "Failed to add to PATH: $($_.Exception.Message)"
        Write-Info "You may need to manually add $InstallDir to your PATH"
    }
}

# Function to verify installation
function Test-Installation {
    param([string]$InstallDir)
    
    Write-Info "Verifying installation..."
    
    $binaryPath = Join-Path $InstallDir "$BINARY_NAME.exe"
    
    if (Test-Path $binaryPath) {
        try {
            $versionOutput = & $binaryPath version 2>$null
            if ($versionOutput -match "version\s+(.+)") {
                $installedVersion = $matches[1]
                Write-Success "$BINARY_NAME $installedVersion installed successfully"
                
                # Show usage information
                Write-Host ""
                Write-Host "Usage:"
                Write-Host "  $BINARY_NAME `"your natural language command`""
                Write-Host "  $BINARY_NAME --help"
                Write-Host ""
                Write-Host "Examples:"
                Write-Host "  $BINARY_NAME `"list files by size`""
                Write-Host "  $BINARY_NAME `"find large files in current directory`""
                Write-Host ""
                
                return $true
            }
        }
        catch {
            # Binary exists but might not be in PATH yet
            Write-Success "Binary installed at $binaryPath"
            Write-Info "Restart your shell or open a new terminal to use $BINARY_NAME"
            return $true
        }
    }
    
    Write-Error "Installation verification failed"
    return $false
}

# Function to cleanup temporary files
function Remove-TempFiles {
    Write-Info "Cleaning up temporary files..."
    if (Test-Path $TMP_DIR) {
        Remove-Item -Recurse -Force $TMP_DIR
    }
}

# Function to show help
function Show-Help {
    Write-Host "nl-to-shell installer for Windows"
    Write-Host ""
    Write-Host "Usage: .\install.ps1 [OPTIONS]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -Version VERSION     Install specific version (default: latest)"
    Write-Host "  -InstallDir DIR      Install directory (default: $env:LOCALAPPDATA\Programs\nl-to-shell)"
    Write-Host "  -Help                Show this help message"
    Write-Host ""
    Write-Host "Examples:"
    Write-Host "  .\install.ps1                              Install latest version"
    Write-Host "  .\install.ps1 -Version v1.0.0              Install specific version"
    Write-Host "  .\install.ps1 -InstallDir C:\Tools         Install to custom directory"
}

# Function to check prerequisites
function Test-Prerequisites {
    Write-Info "Checking prerequisites..."
    
    # Check PowerShell version
    if ($PSVersionTable.PSVersion.Major -lt 5) {
        Write-Error "PowerShell 5.0 or later is required"
        exit 1
    }
    
    # Check if running as administrator for system-wide installation
    $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
    
    if ($InstallDir.StartsWith($env:ProgramFiles) -and -not $isAdmin) {
        Write-Warning "Installing to Program Files requires administrator privileges"
        Write-Info "Consider running as administrator or using a user directory"
    }
    
    Write-Success "Prerequisites check passed"
}

# Main installation function
function Install-NlToShell {
    if ($Help) {
        Show-Help
        return
    }
    
    Write-Host "nl-to-shell installer for Windows" -ForegroundColor Cyan
    Write-Host "==================================" -ForegroundColor Cyan
    Write-Host ""
    
    try {
        # Run installation steps
        Test-Prerequisites
        $platform = Get-Platform
        
        if (-not $Version) {
            $Version = Get-LatestVersion
        }
        else {
            Write-Info "Using specified version: $Version"
        }
        
        $binaryPath = Download-Binary -Version $Version -Platform $platform
        $targetPath = Install-Binary -SourcePath $binaryPath -InstallDir $InstallDir
        Add-ToPath -InstallDir $InstallDir
        
        if (Test-Installation -InstallDir $InstallDir) {
            Write-Host ""
            Write-Success "Installation completed successfully!" -ForegroundColor Green
            Write-Host ""
            Write-Host "Get started with:"
            Write-Host "  $BINARY_NAME --help"
            Write-Host ""
            Write-Host "Note: If the command is not found, restart your shell or open a new terminal."
        }
        else {
            Write-Error "Installation verification failed"
            exit 1
        }
    }
    catch {
        Write-Error "Installation failed: $($_.Exception.Message)"
        exit 1
    }
    finally {
        Remove-TempFiles
    }
}

# Run the installer
Install-NlToShell