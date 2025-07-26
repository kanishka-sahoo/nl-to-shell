package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// mockPlugin implements the ContextPlugin interface for testing
type mockPlugin struct {
	name     string
	priority int
	data     map[string]interface{}
	err      error
}

func (m *mockPlugin) Name() string {
	return m.name
}

func (m *mockPlugin) Priority() int {
	return m.priority
}

func (m *mockPlugin) GatherContext(ctx context.Context, baseContext *types.Context) (map[string]interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.data, nil
}

func TestNewGatherer(t *testing.T) {
	gatherer := NewGatherer()
	if gatherer == nil {
		t.Fatal("NewGatherer() returned nil")
	}

	// Test that it implements the interface
	var _ interfaces.ContextGatherer = gatherer
}

func TestNewGathererWithLimits(t *testing.T) {
	maxFiles := 500
	maxDepth := 2
	gatherer := NewGathererWithLimits(maxFiles, maxDepth).(*Gatherer)

	if gatherer.maxFileListSize != maxFiles {
		t.Errorf("Expected maxFileListSize %d, got %d", maxFiles, gatherer.maxFileListSize)
	}

	if gatherer.maxDepth != maxDepth {
		t.Errorf("Expected maxDepth %d, got %d", maxDepth, gatherer.maxDepth)
	}
}

func TestRegisterPlugin(t *testing.T) {
	gatherer := NewGatherer().(*Gatherer)

	// Test registering a valid plugin
	plugin1 := &mockPlugin{name: "test1", priority: 10}
	err := gatherer.RegisterPlugin(plugin1)
	if err != nil {
		t.Errorf("Failed to register plugin: %v", err)
	}

	plugins := gatherer.GetPluginManager().GetPlugins()
	if len(plugins) != 1 {
		t.Errorf("Expected 1 plugin, got %d", len(plugins))
	}

	// Test registering another plugin with different priority
	plugin2 := &mockPlugin{name: "test2", priority: 20}
	err = gatherer.RegisterPlugin(plugin2)
	if err != nil {
		t.Errorf("Failed to register second plugin: %v", err)
	}

	plugins = gatherer.GetPluginManager().GetPlugins()
	if len(plugins) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(plugins))
	}

	// Check that plugins are sorted by priority (higher first)
	if plugins[0].Priority() != 20 {
		t.Errorf("Expected first plugin to have priority 20, got %d", plugins[0].Priority())
	}

	if plugins[1].Priority() != 10 {
		t.Errorf("Expected second plugin to have priority 10, got %d", plugins[1].Priority())
	}
}

func TestRegisterPluginErrors(t *testing.T) {
	gatherer := NewGatherer().(*Gatherer)

	// Test registering nil plugin
	err := gatherer.RegisterPlugin(nil)
	if err == nil {
		t.Error("Expected error when registering nil plugin")
	}

	// Test registering duplicate plugin name
	plugin1 := &mockPlugin{name: "duplicate", priority: 10}
	err = gatherer.RegisterPlugin(plugin1)
	if err != nil {
		t.Errorf("Failed to register first plugin: %v", err)
	}

	plugin2 := &mockPlugin{name: "duplicate", priority: 20}
	err = gatherer.RegisterPlugin(plugin2)
	if err == nil {
		t.Error("Expected error when registering plugin with duplicate name")
	}
}

func TestGatherContext(t *testing.T) {
	gatherer := NewGatherer().(*Gatherer)
	ctx := context.Background()

	context, err := gatherer.GatherContext(ctx)
	if err != nil {
		t.Errorf("GatherContext failed: %v", err)
	}

	if context == nil {
		t.Fatal("GatherContext returned nil context")
	}

	// Check that working directory is set
	if context.WorkingDirectory == "" {
		t.Error("Working directory not set")
	}

	// Check that files slice is initialized
	if context.Files == nil {
		t.Error("Files slice not initialized")
	}

	// Check that environment map is initialized
	if context.Environment == nil {
		t.Error("Environment map not initialized")
	}

	// Check that plugin data map is initialized
	if context.PluginData == nil {
		t.Error("PluginData map not initialized")
	}
}

func TestGatherContextWithPlugins(t *testing.T) {
	gatherer := NewGatherer().(*Gatherer)
	ctx := context.Background()

	// Register a mock plugin
	pluginData := map[string]interface{}{
		"test_key": "test_value",
	}
	plugin := &mockPlugin{
		name:     "test_plugin",
		priority: 10,
		data:     pluginData,
	}

	err := gatherer.RegisterPlugin(plugin)
	if err != nil {
		t.Errorf("Failed to register plugin: %v", err)
	}

	context, err := gatherer.GatherContext(ctx)
	if err != nil {
		t.Errorf("GatherContext failed: %v", err)
	}

	// Check that plugin data was included
	if context.PluginData["test_plugin"] == nil {
		t.Error("Plugin data not included in context")
	}

	pluginResult := context.PluginData["test_plugin"].(map[string]interface{})
	if pluginResult["test_key"] != "test_value" {
		t.Error("Plugin data not correctly included")
	}
}

func TestGatherContextWithFailingPlugin(t *testing.T) {
	gatherer := NewGatherer().(*Gatherer)
	ctx := context.Background()

	// Register a plugin that will fail
	plugin := &mockPlugin{
		name:     "failing_plugin",
		priority: 10,
		err:      &types.NLShellError{Type: types.ErrTypeValidation, Message: "plugin error"},
	}

	err := gatherer.RegisterPlugin(plugin)
	if err != nil {
		t.Errorf("Failed to register plugin: %v", err)
	}

	// Context gathering should still succeed even if plugin fails
	context, err := gatherer.GatherContext(ctx)
	if err != nil {
		t.Errorf("GatherContext failed: %v", err)
	}

	// Plugin data should not be included for failed plugin
	if context.PluginData["failing_plugin"] != nil {
		t.Error("Failed plugin data should not be included")
	}
}

func TestGatherContextCancellation(t *testing.T) {
	gatherer := NewGatherer().(*Gatherer)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context immediately
	cancel()

	_, err := gatherer.GatherContext(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestScanFileSystem(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir := t.TempDir()

	// Create some test files and directories
	testFiles := []string{
		"file1.txt",
		"file2.go",
		"subdir/file3.py",
		"subdir/file4.js",
		"subdir/nested/file5.md",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tempDir, file)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		err = os.WriteFile(fullPath, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	gatherer := NewGatherer().(*Gatherer)
	ctx := context.Background()

	files, err := gatherer.scanFileSystem(ctx, tempDir)
	if err != nil {
		t.Errorf("scanFileSystem failed: %v", err)
	}

	if len(files) == 0 {
		t.Error("No files found in scan")
	}

	// Check that we found some of our test files
	foundFiles := make(map[string]bool)
	for _, file := range files {
		foundFiles[file.Name] = true
	}

	expectedFiles := []string{"file1.txt", "file2.go", "subdir"}
	for _, expected := range expectedFiles {
		if !foundFiles[expected] {
			t.Errorf("Expected file %s not found in scan results", expected)
		}
	}
}

func TestScanFileSystemWithLimits(t *testing.T) {
	// Create a temporary directory with many files
	tempDir := t.TempDir()

	// Create more files than our limit
	maxFiles := 5
	for i := 0; i < maxFiles+10; i++ {
		filename := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		err := os.WriteFile(filename, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	gatherer := NewGathererWithLimits(maxFiles, 1).(*Gatherer)
	ctx := context.Background()

	files, err := gatherer.scanFileSystem(ctx, tempDir)
	if err != nil {
		t.Errorf("scanFileSystem failed: %v", err)
	}

	// Should not exceed our limit
	if len(files) > maxFiles {
		t.Errorf("Expected at most %d files, got %d", maxFiles, len(files))
	}
}

func TestGatherEnvironment(t *testing.T) {
	gatherer := NewGatherer().(*Gatherer)

	// Set a test environment variable
	testVar := "TEST_NL_SHELL_VAR"
	testValue := "test_value"
	os.Setenv(testVar, testValue)
	defer os.Unsetenv(testVar)

	env := gatherer.gatherEnvironment()

	// Should include common environment variables if they exist
	if path := os.Getenv("PATH"); path != "" {
		if env["PATH"] != path {
			t.Error("PATH environment variable not correctly gathered")
		}
	}

	// Should not include our test variable (not in the relevant list)
	if env[testVar] == testValue {
		t.Error("Non-relevant environment variable should not be included")
	}
}

func TestFileInfoStructure(t *testing.T) {
	// Create a test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "test content"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	gatherer := NewGatherer().(*Gatherer)
	ctx := context.Background()

	files, err := gatherer.scanFileSystem(ctx, tempDir)
	if err != nil {
		t.Errorf("scanFileSystem failed: %v", err)
	}

	// Find our test file
	var testFileInfo *types.FileInfo
	for _, file := range files {
		if file.Name == "test.txt" {
			testFileInfo = &file
			break
		}
	}

	if testFileInfo == nil {
		t.Fatal("Test file not found in scan results")
	}

	// Verify file info structure
	if testFileInfo.IsDir {
		t.Error("Test file incorrectly marked as directory")
	}

	if testFileInfo.Size != int64(len(testContent)) {
		t.Errorf("Expected file size %d, got %d", len(testContent), testFileInfo.Size)
	}

	if testFileInfo.Path != testFile {
		t.Errorf("Expected file path %s, got %s", testFile, testFileInfo.Path)
	}

	// ModTime should be recent
	if time.Since(testFileInfo.ModTime) > time.Minute {
		t.Error("File modification time seems incorrect")
	}
}
func TestGetPluginManager(t *testing.T) {
	gatherer := NewGatherer().(*Gatherer)

	pm := gatherer.GetPluginManager()
	if pm == nil {
		t.Error("GetPluginManager() returned nil")
	}

	// Test that it's the same instance
	if pm != gatherer.pluginManager {
		t.Error("GetPluginManager() returned different instance")
	}
}

func TestLoadPlugins(t *testing.T) {
	gatherer := NewGatherer().(*Gatherer)
	tempDir := t.TempDir()

	// Test loading from empty directory
	err := gatherer.LoadPlugins(tempDir)
	if err != nil {
		t.Errorf("LoadPlugins failed: %v", err)
	}

	// Test loading from non-existent directory
	err = gatherer.LoadPlugins("/non/existent/directory")
	if err != nil {
		t.Errorf("LoadPlugins with non-existent directory should not error: %v", err)
	}
}

func TestGathererPluginIntegration(t *testing.T) {
	gatherer := NewGatherer().(*Gatherer)
	ctx := context.Background()

	// Register a plugin through the gatherer
	plugin := &mockPlugin{
		name:     "integration_test",
		priority: 50,
		data:     map[string]interface{}{"test": "data"},
	}

	err := gatherer.RegisterPlugin(plugin)
	if err != nil {
		t.Errorf("Failed to register plugin: %v", err)
	}

	// Gather context and verify plugin data is included
	context, err := gatherer.GatherContext(ctx)
	if err != nil {
		t.Errorf("GatherContext failed: %v", err)
	}

	if context.PluginData["integration_test"] == nil {
		t.Error("Plugin data not included in context")
	}

	pluginResult := context.PluginData["integration_test"].(map[string]interface{})
	if pluginResult["test"] != "data" {
		t.Error("Plugin data not correctly included")
	}
}
