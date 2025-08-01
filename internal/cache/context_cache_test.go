package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

func TestContextCache_FileSystemContext(t *testing.T) {
	cache := NewContextCache()
	defer cache.Close()

	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "context_cache_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test cache miss
	fsContext, exists := cache.GetFileSystemContext(tempDir, 100, 3)
	if exists {
		t.Fatal("Should not have cached file system context initially")
	}

	// Create and cache file system context
	dirInfo, err := os.Stat(tempDir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	fsContext = &FileSystemContext{
		WorkingDir: tempDir,
		Files: []types.FileInfo{
			{
				Name:    "test.txt",
				Path:    testFile,
				IsDir:   false,
				Size:    12,
				ModTime: time.Now(),
			},
		},
		MaxFiles:         100,
		MaxDepth:         3,
		ScannedAt:        time.Now(),
		DirectoryModTime: dirInfo.ModTime(),
	}

	err = cache.SetFileSystemContext(tempDir, 100, 3, fsContext)
	if err != nil {
		t.Fatalf("Failed to cache file system context: %v", err)
	}

	// Test cache hit
	cachedContext, exists := cache.GetFileSystemContext(tempDir, 100, 3)
	if !exists {
		t.Fatal("Should have cached file system context")
	}

	if cachedContext.WorkingDir != tempDir {
		t.Fatalf("Expected working dir %s, got %s", tempDir, cachedContext.WorkingDir)
	}

	if len(cachedContext.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(cachedContext.Files))
	}

	if cachedContext.Files[0].Name != "test.txt" {
		t.Fatalf("Expected file name test.txt, got %s", cachedContext.Files[0].Name)
	}
}

func TestContextCache_GitContext(t *testing.T) {
	cache := NewContextCache()
	defer cache.Close()

	workingDir := "/test/repo"
	gitContext := &types.GitContext{
		IsRepository:          true,
		CurrentBranch:         "main",
		WorkingTreeStatus:     "clean",
		HasUncommittedChanges: false,
	}

	// Test cache miss
	_, exists := cache.GetGitContext(workingDir)
	if exists {
		t.Fatal("Should not have cached git context initially")
	}

	// Cache git context
	err := cache.SetGitContext(workingDir, gitContext)
	if err != nil {
		t.Fatalf("Failed to cache git context: %v", err)
	}

	// Test cache hit
	cachedContext, exists := cache.GetGitContext(workingDir)
	if !exists {
		t.Fatal("Should have cached git context")
	}

	if cachedContext.CurrentBranch != "main" {
		t.Fatalf("Expected branch main, got %s", cachedContext.CurrentBranch)
	}

	if cachedContext.WorkingTreeStatus != "clean" {
		t.Fatalf("Expected status clean, got %s", cachedContext.WorkingTreeStatus)
	}
}

func TestContextCache_PluginContext(t *testing.T) {
	cache := NewContextCache()
	defer cache.Close()

	pluginName := "test_plugin"
	workingDir := "/test/dir"
	pluginData := map[string]interface{}{
		"version": "1.0.0",
		"tools":   []string{"docker", "node"},
		"config":  map[string]string{"env": "development"},
	}

	// Test cache miss
	_, exists := cache.GetPluginContext(pluginName, workingDir)
	if exists {
		t.Fatal("Should not have cached plugin context initially")
	}

	// Cache plugin context
	err := cache.SetPluginContext(pluginName, workingDir, pluginData)
	if err != nil {
		t.Fatalf("Failed to cache plugin context: %v", err)
	}

	// Test cache hit
	cachedData, exists := cache.GetPluginContext(pluginName, workingDir)
	if !exists {
		t.Fatal("Should have cached plugin context")
	}

	if cachedData["version"] != "1.0.0" {
		t.Fatalf("Expected version 1.0.0, got %v", cachedData["version"])
	}

	tools, ok := cachedData["tools"].([]string)
	if !ok {
		t.Fatal("Expected tools to be []string")
	}

	if len(tools) != 2 || tools[0] != "docker" || tools[1] != "node" {
		t.Fatalf("Expected tools [docker, node], got %v", tools)
	}
}

func TestContextCache_EnvironmentContext(t *testing.T) {
	cache := NewContextCache()
	defer cache.Close()

	envData := map[string]string{
		"PATH":     "/usr/bin:/bin",
		"HOME":     "/home/user",
		"SHELL":    "/bin/bash",
		"GOPATH":   "/home/user/go",
		"NODE_ENV": "development",
	}

	// Test cache miss
	_, exists := cache.GetEnvironmentContext()
	if exists {
		t.Fatal("Should not have cached environment context initially")
	}

	// Cache environment context
	err := cache.SetEnvironmentContext(envData)
	if err != nil {
		t.Fatalf("Failed to cache environment context: %v", err)
	}

	// Test cache hit
	cachedData, exists := cache.GetEnvironmentContext()
	if !exists {
		t.Fatal("Should have cached environment context")
	}

	if len(cachedData) != len(envData) {
		t.Fatalf("Expected %d environment variables, got %d", len(envData), len(cachedData))
	}

	for key, expectedValue := range envData {
		if cachedValue, ok := cachedData[key]; !ok || cachedValue != expectedValue {
			t.Fatalf("Expected %s=%s, got %s=%s", key, expectedValue, key, cachedValue)
		}
	}
}

func TestContextCache_InvalidateDirectory(t *testing.T) {
	cache := NewContextCache()
	defer cache.Close()

	workingDir := "/test/dir"

	// Cache some data
	gitContext := &types.GitContext{
		IsRepository:  true,
		CurrentBranch: "main",
	}
	cache.SetGitContext(workingDir, gitContext)

	pluginData := map[string]interface{}{"test": "data"}
	cache.SetPluginContext("test_plugin", workingDir, pluginData)

	// Verify data is cached
	_, exists := cache.GetGitContext(workingDir)
	if !exists {
		t.Fatal("Git context should be cached")
	}

	_, exists = cache.GetPluginContext("test_plugin", workingDir)
	if !exists {
		t.Fatal("Plugin context should be cached")
	}

	// Invalidate directory
	cache.InvalidateDirectory(workingDir)

	// Verify data is no longer cached
	_, exists = cache.GetGitContext(workingDir)
	if exists {
		t.Fatal("Git context should be invalidated")
	}

	_, exists = cache.GetPluginContext("test_plugin", workingDir)
	if exists {
		t.Fatal("Plugin context should be invalidated")
	}
}

func TestContextCache_Stats(t *testing.T) {
	cache := NewContextCache()
	defer cache.Close()

	// Initially empty
	stats := cache.Stats()
	if stats.Entries != 0 {
		t.Fatalf("Expected 0 entries initially, got %d", stats.Entries)
	}

	// Add some entries
	gitContext := &types.GitContext{IsRepository: true}
	cache.SetGitContext("/test/dir1", gitContext)
	cache.SetGitContext("/test/dir2", gitContext)

	envData := map[string]string{"PATH": "/usr/bin"}
	cache.SetEnvironmentContext(envData)

	// Check stats
	stats = cache.Stats()
	if stats.Entries != 3 {
		t.Fatalf("Expected 3 entries, got %d", stats.Entries)
	}

	if stats.TotalSize <= 0 {
		t.Fatal("Total size should be greater than 0")
	}
}

func TestContextCache_Clear(t *testing.T) {
	cache := NewContextCache()
	defer cache.Close()

	// Add some data
	gitContext := &types.GitContext{IsRepository: true}
	cache.SetGitContext("/test/dir", gitContext)

	envData := map[string]string{"PATH": "/usr/bin"}
	cache.SetEnvironmentContext(envData)

	// Verify data exists
	stats := cache.Stats()
	if stats.Entries == 0 {
		t.Fatal("Should have cached entries before clear")
	}

	// Clear cache
	cache.Clear()

	// Verify cache is empty
	stats = cache.Stats()
	if stats.Entries != 0 {
		t.Fatalf("Expected 0 entries after clear, got %d", stats.Entries)
	}

	// Verify specific entries are gone
	_, exists := cache.GetGitContext("/test/dir")
	if exists {
		t.Fatal("Git context should be cleared")
	}

	_, exists = cache.GetEnvironmentContext()
	if exists {
		t.Fatal("Environment context should be cleared")
	}
}
