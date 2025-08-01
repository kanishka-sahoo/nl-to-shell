package updater

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// Installer handles the actual installation of updates
type Installer struct {
	httpClient *http.Client
}

// NewInstaller creates a new installer
func NewInstaller() *Installer {
	return &Installer{
		httpClient: &http.Client{},
	}
}

// InstallUpdate downloads and installs an update
func (i *Installer) InstallUpdate(ctx context.Context, updateInfo *types.UpdateInfo) error {
	if updateInfo.DownloadURL == "" {
		return &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "no download URL provided",
		}
	}

	// Create temporary directory for download
	tempDir, err := os.MkdirTemp("", "nl-to-shell-update-*")
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to create temporary directory",
			Cause:   err,
		}
	}
	defer os.RemoveAll(tempDir)

	// Download the update
	downloadPath := filepath.Join(tempDir, "update")
	if err := i.downloadFile(ctx, updateInfo.DownloadURL, downloadPath); err != nil {
		return err
	}

	// Verify checksum if provided
	if updateInfo.Checksum != "" {
		if err := i.verifyChecksum(ctx, downloadPath, updateInfo.Checksum); err != nil {
			return err
		}
	}

	// Create backup of current executable
	backupPath, err := i.createBackup()
	if err != nil {
		return err
	}

	// Install the update
	if err := i.installBinary(downloadPath); err != nil {
		// Restore backup on failure
		if restoreErr := i.restoreBackup(backupPath); restoreErr != nil {
			return &types.NLShellError{
				Type:    types.ErrTypePermission,
				Message: fmt.Sprintf("installation failed and backup restore failed: %v (original error: %v)", restoreErr, err),
				Cause:   err,
			}
		}
		return err
	}

	// Clean up backup on success
	os.Remove(backupPath)

	return nil
}

// downloadFile downloads a file from the given URL
func (i *Installer) downloadFile(ctx context.Context, url, filepath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: "failed to create download request",
			Cause:   err,
		}
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: "failed to download update",
			Cause:   err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: fmt.Sprintf("download failed with status %d", resp.StatusCode),
		}
	}

	out, err := os.Create(filepath)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to create download file",
			Cause:   err,
		}
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to write download file",
			Cause:   err,
		}
	}

	return nil
}

// verifyChecksum verifies the downloaded file against a checksum
func (i *Installer) verifyChecksum(ctx context.Context, filepath, checksumURL string) error {
	// Download checksum file
	req, err := http.NewRequestWithContext(ctx, "GET", checksumURL, nil)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: "failed to create checksum request",
			Cause:   err,
		}
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: "failed to download checksum",
			Cause:   err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: fmt.Sprintf("checksum download failed with status %d", resp.StatusCode),
		}
	}

	checksumData, err := io.ReadAll(resp.Body)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: "failed to read checksum data",
			Cause:   err,
		}
	}

	expectedChecksum := strings.TrimSpace(string(checksumData))
	// Extract just the hash if it's in "hash filename" format
	if parts := strings.Fields(expectedChecksum); len(parts) >= 1 {
		expectedChecksum = parts[0]
	}

	// Calculate actual checksum
	file, err := os.Open(filepath)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to open file for checksum verification",
			Cause:   err,
		}
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to calculate checksum",
			Cause:   err,
		}
	}

	actualChecksum := fmt.Sprintf("%x", hash.Sum(nil))

	if actualChecksum != expectedChecksum {
		return &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: fmt.Sprintf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum),
		}
	}

	return nil
}

// createBackup creates a backup of the current executable
func (i *Installer) createBackup() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to get current executable path",
			Cause:   err,
		}
	}

	backupPath := execPath + ".backup"

	// Copy current executable to backup
	src, err := os.Open(execPath)
	if err != nil {
		return "", &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to open current executable for backup",
			Cause:   err,
		}
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to create backup file",
			Cause:   err,
		}
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to copy executable to backup",
			Cause:   err,
		}
	}

	// Copy permissions
	srcInfo, err := src.Stat()
	if err != nil {
		return "", &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to get source file permissions",
			Cause:   err,
		}
	}

	if err := dst.Chmod(srcInfo.Mode()); err != nil {
		return "", &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to set backup file permissions",
			Cause:   err,
		}
	}

	return backupPath, nil
}

// restoreBackup restores the backup in case of installation failure
func (i *Installer) restoreBackup(backupPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to get current executable path",
			Cause:   err,
		}
	}

	// Remove the failed update
	os.Remove(execPath)

	// Restore backup
	if err := os.Rename(backupPath, execPath); err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to restore backup",
			Cause:   err,
		}
	}

	return nil
}

// installBinary installs the new binary, handling platform-specific requirements
func (i *Installer) installBinary(downloadPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to get current executable path",
			Cause:   err,
		}
	}

	// Make the downloaded file executable on Unix systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(downloadPath, 0755); err != nil {
			return &types.NLShellError{
				Type:    types.ErrTypePermission,
				Message: "failed to make downloaded file executable",
				Cause:   err,
			}
		}
	}

	// On Windows, we might need to handle the case where the current executable is running
	if runtime.GOOS == "windows" {
		return i.installBinaryWindows(downloadPath, execPath)
	}

	return i.installBinaryUnix(downloadPath, execPath)
}

// installBinaryUnix installs the binary on Unix-like systems
func (i *Installer) installBinaryUnix(downloadPath, execPath string) error {
	// Open source file
	src, err := os.Open(downloadPath)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to open downloaded file",
			Cause:   err,
		}
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(execPath)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to create new executable",
			Cause:   err,
		}
	}
	defer dst.Close()

	// Copy file content
	if _, err := io.Copy(dst, src); err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to copy new executable",
			Cause:   err,
		}
	}

	// Set executable permissions
	if err := dst.Chmod(0755); err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to set executable permissions",
			Cause:   err,
		}
	}

	return nil
}

// installBinaryWindows installs the binary on Windows
func (i *Installer) installBinaryWindows(downloadPath, execPath string) error {
	// On Windows, we can't replace a running executable directly
	// We need to rename the current one and then move the new one in place
	tempExecPath := execPath + ".old"

	// Rename current executable
	if err := os.Rename(execPath, tempExecPath); err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to rename current executable",
			Cause:   err,
		}
	}

	// Move new executable into place
	if err := os.Rename(downloadPath, execPath); err != nil {
		// Try to restore original executable
		os.Rename(tempExecPath, execPath)
		return &types.NLShellError{
			Type:    types.ErrTypePermission,
			Message: "failed to install new executable",
			Cause:   err,
		}
	}

	// Remove old executable
	os.Remove(tempExecPath)

	return nil
}
