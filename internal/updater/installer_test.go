package updater

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

func TestNewInstaller(t *testing.T) {
	installer := NewInstaller()

	if installer == nil {
		t.Error("Expected installer to be created")
	}

	if installer.httpClient == nil {
		t.Error("Expected HTTP client to be initialized")
	}
}

func TestDownloadFile(t *testing.T) {
	// Create test content
	testContent := "test binary content"

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	installer := NewInstaller()
	installer.httpClient = &http.Client{Transport: &testTransport{server: server}}

	// Create temp file for download
	tempDir, err := os.MkdirTemp("", "test-download-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	downloadPath := filepath.Join(tempDir, "downloaded")

	ctx := context.Background()
	err = installer.downloadFile(ctx, server.URL+"/test", downloadPath)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify file was downloaded correctly
	content, err := os.ReadFile(downloadPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Expected content %s, got %s", testContent, string(content))
	}
}

func TestDownloadFile_HTTPError(t *testing.T) {
	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	installer := NewInstaller()
	installer.httpClient = &http.Client{Transport: &testTransport{server: server}}

	tempDir, err := os.MkdirTemp("", "test-download-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	downloadPath := filepath.Join(tempDir, "downloaded")

	ctx := context.Background()
	err = installer.downloadFile(ctx, server.URL+"/test", downloadPath)

	if err == nil {
		t.Error("Expected HTTP error, got nil")
	}

	var nlErr *types.NLShellError
	if !errors.As(err, &nlErr) {
		t.Errorf("Expected NLShellError, got %T", err)
	} else if nlErr.Type != types.ErrTypeNetwork {
		t.Errorf("Expected ErrTypeNetwork, got %v", nlErr.Type)
	}
}

func TestDownloadFile_ContextCancellation(t *testing.T) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	installer := NewInstaller()
	installer.httpClient = &http.Client{Transport: &testTransport{server: server}}

	tempDir, err := os.MkdirTemp("", "test-download-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	downloadPath := filepath.Join(tempDir, "downloaded")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = installer.downloadFile(ctx, server.URL+"/test", downloadPath)

	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}
}

func TestVerifyChecksum_Success(t *testing.T) {
	testContent := "test binary content"
	hash := sha256.Sum256([]byte(testContent))
	expectedChecksum := fmt.Sprintf("%x", hash)

	// Create mock server for checksum
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedChecksum))
	}))
	defer server.Close()

	installer := NewInstaller()
	installer.httpClient = &http.Client{Transport: &testTransport{server: server}}

	// Create temp file with test content
	tempDir, err := os.MkdirTemp("", "test-checksum-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "testfile")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	err = installer.verifyChecksum(ctx, testFile, server.URL+"/checksum")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	testContent := "test binary content"
	wrongChecksum := "0123456789abcdef" // Wrong checksum

	// Create mock server for checksum
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(wrongChecksum))
	}))
	defer server.Close()

	installer := NewInstaller()
	installer.httpClient = &http.Client{Transport: &testTransport{server: server}}

	// Create temp file with test content
	tempDir, err := os.MkdirTemp("", "test-checksum-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "testfile")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	err = installer.verifyChecksum(ctx, testFile, server.URL+"/checksum")

	if err == nil {
		t.Error("Expected checksum mismatch error, got nil")
	}

	var nlErr *types.NLShellError
	if !errors.As(err, &nlErr) {
		t.Errorf("Expected NLShellError, got %T", err)
	} else if nlErr.Type != types.ErrTypeValidation {
		t.Errorf("Expected ErrTypeValidation, got %v", nlErr.Type)
	}
}

func TestVerifyChecksum_WithFilename(t *testing.T) {
	testContent := "test binary content"
	hash := sha256.Sum256([]byte(testContent))
	expectedChecksum := fmt.Sprintf("%x", hash)
	checksumWithFilename := expectedChecksum + "  testfile" // Format: "hash  filename"

	// Create mock server for checksum
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(checksumWithFilename))
	}))
	defer server.Close()

	installer := NewInstaller()
	installer.httpClient = &http.Client{Transport: &testTransport{server: server}}

	// Create temp file with test content
	tempDir, err := os.MkdirTemp("", "test-checksum-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "testfile")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	err = installer.verifyChecksum(ctx, testFile, server.URL+"/checksum")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestCreateBackup(t *testing.T) {
	installer := NewInstaller()

	// This test is tricky because we can't easily mock os.Executable()
	// We'll test the error case where the executable doesn't exist
	// In a real scenario, this would work with the actual executable

	// For now, we'll just test that the function exists and handles errors
	// A more comprehensive test would require setting up a mock executable
	_, err := installer.createBackup()

	// We expect this to work in the test environment, but if it fails
	// it should be a proper NLShellError
	if err != nil {
		var nlErr *types.NLShellError
		if !errors.As(err, &nlErr) {
			t.Errorf("Expected NLShellError, got %T", err)
		}
	}
}

func TestInstallBinaryUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}

	installer := NewInstaller()

	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "test-install-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
	srcPath := filepath.Join(tempDir, "source")
	srcContent := "#!/bin/bash\necho 'test binary'"
	if err := os.WriteFile(srcPath, []byte(srcContent), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create destination file
	dstPath := filepath.Join(tempDir, "destination")

	err = installer.installBinaryUnix(srcPath, dstPath)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify file was copied
	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(content) != srcContent {
		t.Errorf("Expected content %s, got %s", srcContent, string(content))
	}

	// Verify permissions
	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("Failed to stat destination file: %v", err)
	}

	expectedMode := os.FileMode(0755)
	if info.Mode() != expectedMode {
		t.Errorf("Expected mode %v, got %v", expectedMode, info.Mode())
	}
}

func TestInstallBinaryWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows")
	}

	installer := NewInstaller()

	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "test-install-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
	srcPath := filepath.Join(tempDir, "source.exe")
	srcContent := "test binary content"
	if err := os.WriteFile(srcPath, []byte(srcContent), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create existing destination file
	dstPath := filepath.Join(tempDir, "destination.exe")
	oldContent := "old binary content"
	if err := os.WriteFile(dstPath, []byte(oldContent), 0644); err != nil {
		t.Fatalf("Failed to create destination file: %v", err)
	}

	err = installer.installBinaryWindows(srcPath, dstPath)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify file was replaced
	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(content) != srcContent {
		t.Errorf("Expected content %s, got %s", srcContent, string(content))
	}

	// Verify old file was removed
	oldPath := dstPath + ".old"
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Expected old file to be removed")
	}
}

func TestInstallUpdate_Integration(t *testing.T) {
	testContent := "test binary content"
	hash := sha256.Sum256([]byte(testContent))
	expectedChecksum := fmt.Sprintf("%x", hash)

	// Create mock servers
	binaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "checksum") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(expectedChecksum))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testContent))
		}
	}))
	defer binaryServer.Close()

	installer := NewInstaller()
	installer.httpClient = &http.Client{Transport: &testTransport{server: binaryServer}}

	updateInfo := &types.UpdateInfo{
		Available:      true,
		LatestVersion:  "v1.1.0",
		CurrentVersion: "v1.0.0",
		DownloadURL:    binaryServer.URL + "/binary",
		Checksum:       binaryServer.URL + "/checksum",
	}

	ctx := context.Background()

	// Note: This test will fail in the actual installation step because
	// we can't replace the running test executable. In a real scenario,
	// this would work properly. We're testing the download and verification parts.
	err := installer.InstallUpdate(ctx, updateInfo)

	// We expect this to fail at the installation step, but the download
	// and checksum verification should work
	if err != nil {
		// Check if it's a permission error (expected when trying to replace running executable)
		var nlErr *types.NLShellError
		if errors.As(err, &nlErr) && nlErr.Type == types.ErrTypePermission {
			// This is expected - we can't replace the running test executable
			t.Logf("Expected permission error when trying to replace running executable: %v", err)
		} else {
			t.Errorf("Unexpected error type: %v", err)
		}
	}
}

func TestInstallUpdate_NoDownloadURL(t *testing.T) {
	installer := NewInstaller()

	updateInfo := &types.UpdateInfo{
		Available:      true,
		LatestVersion:  "v1.1.0",
		CurrentVersion: "v1.0.0",
		// No DownloadURL
	}

	ctx := context.Background()
	err := installer.InstallUpdate(ctx, updateInfo)

	if err == nil {
		t.Error("Expected error for missing download URL, got nil")
	}

	var nlErr *types.NLShellError
	if !errors.As(err, &nlErr) {
		t.Errorf("Expected NLShellError, got %T", err)
	} else if nlErr.Type != types.ErrTypeValidation {
		t.Errorf("Expected ErrTypeValidation, got %v", nlErr.Type)
	}
}

func TestInstallUpdate_DownloadFailure(t *testing.T) {
	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	installer := NewInstaller()
	installer.httpClient = &http.Client{Transport: &testTransport{server: server}}

	updateInfo := &types.UpdateInfo{
		Available:      true,
		LatestVersion:  "v1.1.0",
		CurrentVersion: "v1.0.0",
		DownloadURL:    server.URL + "/binary",
	}

	ctx := context.Background()
	err := installer.InstallUpdate(ctx, updateInfo)

	if err == nil {
		t.Error("Expected download error, got nil")
	}

	var nlErr *types.NLShellError
	if !errors.As(err, &nlErr) {
		t.Errorf("Expected NLShellError, got %T", err)
	} else if nlErr.Type != types.ErrTypeNetwork {
		t.Errorf("Expected ErrTypeNetwork, got %v", nlErr.Type)
	}
}
