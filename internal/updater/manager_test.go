package updater

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

func TestNewManager(t *testing.T) {
	manager := NewManager("v1.0.0", "owner", "repo")

	if manager.currentVersion != "v1.0.0" {
		t.Errorf("Expected current version v1.0.0, got %s", manager.currentVersion)
	}

	if manager.repoOwner != "owner" {
		t.Errorf("Expected repo owner 'owner', got %s", manager.repoOwner)
	}

	if manager.repoName != "repo" {
		t.Errorf("Expected repo name 'repo', got %s", manager.repoName)
	}

	if manager.httpClient == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if manager.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", manager.httpClient.Timeout)
	}
}

func TestGetCurrentVersion(t *testing.T) {
	manager := NewManager("v1.2.3", "owner", "repo")

	version := manager.GetCurrentVersion()
	if version != "v1.2.3" {
		t.Errorf("Expected version v1.2.3, got %s", version)
	}
}

func TestIsNewerVersion(t *testing.T) {
	manager := NewManager("v1.0.0", "owner", "repo")

	tests := []struct {
		name     string
		latest   string
		current  string
		expected bool
	}{
		{
			name:     "newer version available",
			latest:   "v1.1.0",
			current:  "v1.0.0",
			expected: true,
		},
		{
			name:     "same version",
			latest:   "v1.0.0",
			current:  "v1.0.0",
			expected: false,
		},
		{
			name:     "older version",
			latest:   "v0.9.0",
			current:  "v1.0.0",
			expected: false,
		},
		{
			name:     "version without v prefix",
			latest:   "1.1.0",
			current:  "1.0.0",
			expected: true,
		},
		{
			name:     "mixed version formats",
			latest:   "v1.1.0",
			current:  "1.0.0",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.isNewerVersion(tt.latest, tt.current)
			if result != tt.expected {
				t.Errorf("isNewerVersion(%s, %s) = %v, expected %v",
					tt.latest, tt.current, result, tt.expected)
			}
		})
	}
}

func TestFindAssetForPlatform(t *testing.T) {
	manager := NewManager("v1.0.0", "owner", "repo")

	assets := []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	}{
		{
			Name:               "app-linux-amd64",
			BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/app-linux-amd64",
		},
		{
			Name:               "app-linux-amd64.sha256",
			BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/app-linux-amd64.sha256",
		},
		{
			Name:               "app-darwin-amd64",
			BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/app-darwin-amd64",
		},
		{
			Name:               "app-darwin-arm64",
			BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/app-darwin-arm64",
		},
		{
			Name:               "app-windows-amd64.exe",
			BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/app-windows-amd64.exe",
		},
	}

	downloadURL, checksum := manager.findAssetForPlatform(assets)

	// Verify that we get a download URL for the current platform
	if downloadURL == "" {
		t.Error("Expected download URL to be found for current platform")
	}

	// Verify the URL contains the correct platform identifier
	expectedPattern := runtime.GOOS + "-" + runtime.GOARCH
	if !strings.Contains(downloadURL, expectedPattern) {
		t.Errorf("Expected download URL to contain %s, got %s", expectedPattern, downloadURL)
	}

	// For Linux amd64, we should also get a checksum URL
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		if checksum == "" {
			t.Error("Expected checksum URL to be found for Linux amd64")
		}
		if !strings.Contains(checksum, "sha256") {
			t.Errorf("Expected checksum URL to contain sha256, got %s", checksum)
		}
	}
}

func TestCheckForUpdates_Success(t *testing.T) {
	// Create a mock GitHub API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		expectedPath := "/repos/owner/repo/releases/latest"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		userAgent := r.Header.Get("User-Agent")
		if !strings.Contains(userAgent, "repo/v1.0.0") {
			t.Errorf("Expected User-Agent to contain repo/v1.0.0, got %s", userAgent)
		}

		// Return mock release data
		release := GitHubRelease{
			TagName:     "v1.1.0",
			Name:        "Release v1.1.0",
			Body:        "Bug fixes and improvements",
			Prerelease:  false,
			Draft:       false,
			PublishedAt: "2023-01-01T00:00:00Z",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
				Size               int64  `json:"size"`
			}{
				{
					Name:               "app-linux-amd64",
					BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.1.0/app-linux-amd64",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	manager := NewManager("v1.0.0", "owner", "repo")
	// Create a custom transport that redirects to our test server
	manager.httpClient = &http.Client{
		Timeout:   30 * time.Second,
		Transport: &testTransport{server: server},
	}

	ctx := context.Background()
	updateInfo, err := manager.CheckForUpdates(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !updateInfo.Available {
		t.Error("Expected update to be available")
	}

	if updateInfo.LatestVersion != "v1.1.0" {
		t.Errorf("Expected latest version v1.1.0, got %s", updateInfo.LatestVersion)
	}

	if updateInfo.CurrentVersion != "v1.0.0" {
		t.Errorf("Expected current version v1.0.0, got %s", updateInfo.CurrentVersion)
	}

	if updateInfo.ReleaseNotes != "Bug fixes and improvements" {
		t.Errorf("Expected release notes 'Bug fixes and improvements', got %s", updateInfo.ReleaseNotes)
	}
}

func TestCheckForUpdates_NoUpdateAvailable(t *testing.T) {
	// Create a mock GitHub API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return same version as current
		release := GitHubRelease{
			TagName:     "v1.0.0",
			Name:        "Release v1.0.0",
			Body:        "Initial release",
			Prerelease:  false,
			Draft:       false,
			PublishedAt: "2023-01-01T00:00:00Z",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	manager := NewManager("v1.0.0", "owner", "repo")
	manager.httpClient = &http.Client{
		Timeout:   30 * time.Second,
		Transport: &testTransport{server: server},
	}

	ctx := context.Background()
	updateInfo, err := manager.CheckForUpdates(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if updateInfo.Available {
		t.Error("Expected no update to be available")
	}

	if updateInfo.LatestVersion != "v1.0.0" {
		t.Errorf("Expected latest version v1.0.0, got %s", updateInfo.LatestVersion)
	}
}

func TestCheckForUpdates_DraftRelease(t *testing.T) {
	// Create a mock GitHub API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return draft release
		release := GitHubRelease{
			TagName:     "v1.1.0",
			Name:        "Release v1.1.0",
			Body:        "Draft release",
			Prerelease:  false,
			Draft:       true, // This is a draft
			PublishedAt: "2023-01-01T00:00:00Z",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	manager := NewManager("v1.0.0", "owner", "repo")
	manager.httpClient = &http.Client{
		Timeout:   30 * time.Second,
		Transport: &testTransport{server: server},
	}

	ctx := context.Background()
	updateInfo, err := manager.CheckForUpdates(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if updateInfo.Available {
		t.Error("Expected no update to be available for draft release")
	}
}

func TestCheckForUpdates_NetworkError(t *testing.T) {
	manager := NewManager("v1.0.0", "owner", "repo")
	// Use an invalid URL to trigger network error
	manager.httpClient = &http.Client{
		Timeout: 1 * time.Millisecond, // Very short timeout
	}

	ctx := context.Background()
	_, err := manager.CheckForUpdates(ctx)

	if err == nil {
		t.Error("Expected network error, got nil")
	}

	var nlErr *types.NLShellError
	if !errors.As(err, &nlErr) {
		t.Errorf("Expected NLShellError, got %T", err)
	} else if nlErr.Type != types.ErrTypeNetwork {
		t.Errorf("Expected ErrTypeNetwork, got %v", nlErr.Type)
	}
}

func TestCheckForUpdates_HTTPError(t *testing.T) {
	// Create a mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	manager := NewManager("v1.0.0", "owner", "repo")
	manager.httpClient = &http.Client{
		Timeout:   30 * time.Second,
		Transport: &testTransport{server: server},
	}

	ctx := context.Background()
	_, err := manager.CheckForUpdates(ctx)

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

func TestCheckForUpdates_InvalidJSON(t *testing.T) {
	// Create a mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	manager := NewManager("v1.0.0", "owner", "repo")
	manager.httpClient = &http.Client{
		Timeout:   30 * time.Second,
		Transport: &testTransport{server: server},
	}

	ctx := context.Background()
	_, err := manager.CheckForUpdates(ctx)

	if err == nil {
		t.Error("Expected JSON parsing error, got nil")
	}

	var nlErr *types.NLShellError
	if !errors.As(err, &nlErr) {
		t.Errorf("Expected NLShellError, got %T", err)
	} else if nlErr.Type != types.ErrTypeValidation {
		t.Errorf("Expected ErrTypeValidation, got %v", nlErr.Type)
	}
}

func TestCheckForUpdates_ContextCancellation(t *testing.T) {
	// Create a mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := NewManager("v1.0.0", "owner", "repo")
	manager.httpClient = &http.Client{
		Timeout:   30 * time.Second,
		Transport: &testTransport{server: server},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := manager.CheckForUpdates(ctx)

	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}
}

func TestPerformUpdate_NoUpdateAvailable(t *testing.T) {
	manager := NewManager("v1.0.0", "owner", "repo")

	updateInfo := &types.UpdateInfo{
		Available:      false, // No update available
		LatestVersion:  "v1.0.0",
		CurrentVersion: "v1.0.0",
	}

	ctx := context.Background()
	err := manager.PerformUpdate(ctx, updateInfo)

	if err == nil {
		t.Error("Expected error for no update available, got nil")
	}

	var nlErr *types.NLShellError
	if !errors.As(err, &nlErr) {
		t.Errorf("Expected NLShellError, got %T", err)
	} else if nlErr.Type != types.ErrTypeValidation {
		t.Errorf("Expected ErrTypeValidation, got %v", nlErr.Type)
	}
}

func TestPerformUpdate_MissingDownloadURL(t *testing.T) {
	manager := NewManager("v1.0.0", "owner", "repo")

	updateInfo := &types.UpdateInfo{
		Available:      true,
		LatestVersion:  "v1.1.0",
		CurrentVersion: "v1.0.0",
		// Missing DownloadURL
	}

	ctx := context.Background()
	err := manager.PerformUpdate(ctx, updateInfo)

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

// testTransport is a custom HTTP transport for testing that redirects requests to a test server
type testTransport struct {
	server *httptest.Server
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Parse the test server URL
	serverURL, err := url.Parse(t.server.URL)
	if err != nil {
		return nil, err
	}

	// Redirect the request to our test server
	req.URL.Scheme = serverURL.Scheme
	req.URL.Host = serverURL.Host

	// Use the default transport to actually make the request
	return http.DefaultTransport.RoundTrip(req)
}
