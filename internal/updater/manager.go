package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// Manager implements the UpdateManager interface
type Manager struct {
	currentVersion string
	repoOwner      string
	repoName       string
	httpClient     *http.Client
}

// NewManager creates a new update manager
func NewManager(currentVersion, repoOwner, repoName string) *Manager {
	return &Manager{
		currentVersion: currentVersion,
		repoOwner:      repoOwner,
		repoName:       repoName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GitHubRelease represents a GitHub release response
type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// CheckForUpdates checks if updates are available from GitHub releases
func (m *Manager) CheckForUpdates(ctx context.Context) (*types.UpdateInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", m.repoOwner, m.repoName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: "failed to create request for update check",
			Cause:   err,
		}
	}

	// Set User-Agent header as required by GitHub API
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", m.repoName, m.currentVersion))

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: "failed to check for updates",
			Cause:   err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: fmt.Sprintf("GitHub API returned status %d", resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeNetwork,
			Message: "failed to read response body",
			Cause:   err,
		}
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "failed to parse GitHub release response",
			Cause:   err,
		}
	}

	// Skip draft releases
	if release.Draft {
		return &types.UpdateInfo{
			Available:      false,
			CurrentVersion: m.currentVersion,
		}, nil
	}

	// Determine if update is available
	available := m.isNewerVersion(release.TagName, m.currentVersion)

	updateInfo := &types.UpdateInfo{
		Available:      available,
		LatestVersion:  release.TagName,
		CurrentVersion: m.currentVersion,
		ReleaseNotes:   release.Body,
	}

	if available {
		// Find appropriate asset for current platform
		downloadURL, checksum := m.findAssetForPlatform(release.Assets)
		updateInfo.DownloadURL = downloadURL
		updateInfo.Checksum = checksum
	}

	return updateInfo, nil
}

// PerformUpdate performs the actual update installation
func (m *Manager) PerformUpdate(ctx context.Context, updateInfo *types.UpdateInfo) error {
	if !updateInfo.Available {
		return &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "no update available",
		}
	}

	installer := NewInstaller()
	return installer.InstallUpdate(ctx, updateInfo)
}

// GetCurrentVersion returns the current version of the application
func (m *Manager) GetCurrentVersion() string {
	return m.currentVersion
}

// isNewerVersion compares two version strings and returns true if latest is newer than current
func (m *Manager) isNewerVersion(latest, current string) bool {
	// Remove 'v' prefix if present
	latest = strings.TrimPrefix(latest, "v")
	current = strings.TrimPrefix(current, "v")

	// Simple string comparison for now - in a real implementation,
	// we would use semantic versioning comparison
	return latest != current && latest > current
}

// findAssetForPlatform finds the appropriate download asset for the current platform
func (m *Manager) findAssetForPlatform(assets []struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}) (downloadURL, checksum string) {
	// Determine platform-specific asset name pattern
	var pattern string
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			pattern = "linux-amd64"
		case "arm64":
			pattern = "linux-arm64"
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			pattern = "darwin-amd64"
		case "arm64":
			pattern = "darwin-arm64"
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			pattern = "windows-amd64"
		}
	}

	// Find matching asset
	for _, asset := range assets {
		if strings.Contains(asset.Name, pattern) && !strings.Contains(asset.Name, ".sha256") {
			downloadURL = asset.BrowserDownloadURL

			// Look for corresponding checksum file
			checksumName := asset.Name + ".sha256"
			for _, checksumAsset := range assets {
				if checksumAsset.Name == checksumName {
					checksum = checksumAsset.BrowserDownloadURL
					break
				}
			}
			break
		}
	}

	return downloadURL, checksum
}
