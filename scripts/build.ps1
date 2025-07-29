# PowerShell build script for nl-to-shell
# Supports cross-platform builds with version embedding

param(
    [Parameter(Position=0)]
    [ValidateSet("current", "all", "clean", "info", "verify")]
    [string]$Command = "all"
)

# Build configuration
$BINARY_NAME = "nl-to-shell"
$BUILD_DIR = "bin"
$CMD_DIR = "cmd/nl-to-shell"

# Version information
$VERSION = if ($env:VERSION) { $env:VERSION } else { "0.1.0-dev" }
$GIT_COMMIT = if ($env:GIT_COMMIT) { $env:GIT_COMMIT } else { 
    try { 
        (git rev-parse --short HEAD 2>$null) 
    } catch { 
        "unknown" 
    } 
}
$BUILD_DATE = if ($env:BUILD_DATE) { $env:BUILD_DATE } else { (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ") }

# Go build flags for static linking and optimization
$LDFLAGS = "-w -s -X 'github.com/nl-to-shell/nl-to-shell/internal/cli.Version=$VERSION' -X 'github.com/nl-to-shell/nl-to-shell/internal/cli.GitCommit=$GIT_COMMIT' -X 'github.com/nl-to-shell/nl-to-shell/internal/cli.BuildDate=$BUILD_DATE'"

# CGO settings for static builds
$env:CGO_ENABLED = "0"

# Target platforms
$PLATFORMS = @(
    "linux/amd64",
    "linux/arm64",
    "darwin/amd64", 
    "darwin/arm64",
    "windows/amd64"
)

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

# Function to build for a specific platform
function Build-Platform {
    param([string]$Platform)
    
    $parts = $Platform.Split('/')
    $goos = $parts[0]
    $goarch = $parts[1]
    
    $output_name = "$BINARY_NAME-$goos-$goarch"
    if ($goos -eq "windows") {
        $output_name = "$output_name.exe"
    }
    
    $output_path = "$BUILD_DIR/$output_name"
    
    Write-Info "Building for $goos/$goarch..."
    
    $env:GOOS = $goos
    $env:GOARCH = $goarch
    
    $buildArgs = @(
        "build",
        "-ldflags=$LDFLAGS",
        "-o", $output_path,
        "./$CMD_DIR"
    )
    
    & go @buildArgs
    
    if ($LASTEXITCODE -eq 0) {
        $size = (Get-Item $output_path).Length
        $sizeKB = [math]::Round($size / 1KB, 1)
        Write-Success "Built $output_name ($sizeKB KB)"
        
        # Create checksum
        try {
            $hash = Get-FileHash -Path $output_path -Algorithm SHA256
            $hash.Hash.ToLower() + "  " + (Split-Path $output_path -Leaf) | Out-File -FilePath "$output_path.sha256" -Encoding ASCII
        } catch {
            Write-Warning "Failed to generate checksum for $output_name"
        }
    } else {
        Write-Error "Failed to build for $goos/$goarch"
        return $false
    }
    
    return $true
}

# Function to build for current platform only
function Build-Current {
    Write-Info "Building for current platform..."
    
    $buildArgs = @(
        "build",
        "-ldflags=$LDFLAGS", 
        "-o", "$BUILD_DIR/$BINARY_NAME.exe",
        "./$CMD_DIR"
    )
    
    & go @buildArgs
    
    if ($LASTEXITCODE -eq 0) {
        $size = (Get-Item "$BUILD_DIR/$BINARY_NAME.exe").Length
        $sizeKB = [math]::Round($size / 1KB, 1)
        Write-Success "Built $BINARY_NAME.exe ($sizeKB KB)"
        return $true
    } else {
        Write-Error "Failed to build for current platform"
        return $false
    }
}

# Function to clean build artifacts
function Clean-Build {
    Write-Info "Cleaning build artifacts..."
    if (Test-Path $BUILD_DIR) {
        Remove-Item -Recurse -Force $BUILD_DIR
    }
    Write-Success "Build artifacts cleaned"
}

# Function to show build information
function Show-Info {
    Write-Host "Build Information:"
    Write-Host "  Version: $VERSION"
    Write-Host "  Git Commit: $GIT_COMMIT"
    Write-Host "  Build Date: $BUILD_DATE"
    Write-Host "  Binary Name: $BINARY_NAME"
    Write-Host "  Build Directory: $BUILD_DIR"
    Write-Host ""
    Write-Host "Supported Platforms:"
    foreach ($platform in $PLATFORMS) {
        Write-Host "  - $platform"
    }
}

# Function to verify Go environment
function Verify-Environment {
    Write-Info "Verifying build environment..."
    
    # Check Go installation
    try {
        $goVersion = & go version
        Write-Info "Go version: $goVersion"
    } catch {
        Write-Error "Go is not installed or not in PATH"
        exit 1
    }
    
    # Check if we're in a Go module
    if (-not (Test-Path "go.mod")) {
        Write-Error "go.mod not found. Please run from the project root directory."
        exit 1
    }
    
    # Create build directory
    if (-not (Test-Path $BUILD_DIR)) {
        New-Item -ItemType Directory -Path $BUILD_DIR | Out-Null
    }
    
    Write-Success "Environment verification complete"
}

# Main execution
switch ($Command) {
    "current" {
        Verify-Environment
        Show-Info
        Build-Current
    }
    "all" {
        Verify-Environment
        Show-Info
        Write-Info "Building for all platforms..."
        $success = $true
        foreach ($platform in $PLATFORMS) {
            if (-not (Build-Platform $platform)) {
                $success = $false
            }
        }
        if ($success) {
            Write-Success "All builds completed"
        } else {
            Write-Error "Some builds failed"
            exit 1
        }
    }
    "clean" {
        Clean-Build
    }
    "info" {
        Show-Info
    }
    "verify" {
        Verify-Environment
    }
    default {
        Write-Host "Usage: .\scripts\build.ps1 [current|all|clean|info|verify]"
        Write-Host ""
        Write-Host "Commands:"
        Write-Host "  current  - Build for current platform only"
        Write-Host "  all      - Build for all supported platforms (default)"
        Write-Host "  clean    - Clean build artifacts"
        Write-Host "  info     - Show build information"
        Write-Host "  verify   - Verify build environment"
        exit 1
    }
}