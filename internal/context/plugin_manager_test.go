package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/interfaces"
	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// panicPlugin is a test plugin that panics during execution
type panicPlugin struct {
	name     string
	priority int
}

func (p *panicPlugin) Name() string {
	return p.name
}

func (p *panicPlugin) Priority() int {
	return p.priority
}

func (p *panicPlugin) GatherContext(ctx context.Context, baseContext *types.Context) (map[string]interface{}, error) {
	panic("test panic")
}

// slowPlugin is a test plugin that takes time to execute
type slowPlugin struct {
	name     string
	priority int
	delay    time.Duration
}

func (s *slowPlugin) Name() string {
	return s.name
}

func (s *slowPlugin) Priority() int {
	return s.priority
}

func (s *slowPlugin) GatherContext(ctx context.Context, baseContext *types.Context) (map[string]interface{}, error) {
	select {
	case <-time.After(s.delay):
		return map[string]interface{}{"completed": true}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestNewPluginManager(t *testing.T) {
	pm := NewPluginManager()
	if pm == nil {
		t.Fatal("NewPluginManager() returned nil")
	}

	// Test that it implements the interface
	var _ interfaces.PluginManager = pm
}

func TestPluginManagerRegisterPlugin(t *testing.T) {
	pm := NewPluginManager()

	// Test registering a valid plugin
	plugin1 := &mockPlugin{name: "test1", priority: 10}
	err := pm.RegisterPlugin(plugin1)
	if err != nil {
		t.Errorf("Failed to register plugin: %v", err)
	}

	plugins := pm.GetPlugins()
	if len(plugins) != 1 {
		t.Errorf("Expected 1 plugin, got %d", len(plugins))
	}

	// Test registering another plugin with different priority
	plugin2 := &mockPlugin{name: "test2", priority: 20}
	err = pm.RegisterPlugin(plugin2)
	if err != nil {
		t.Errorf("Failed to register second plugin: %v", err)
	}

	plugins = pm.GetPlugins()
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

func TestPluginManagerRegisterPluginErrors(t *testing.T) {
	pm := NewPluginManager()

	// Test registering nil plugin
	err := pm.RegisterPlugin(nil)
	if err == nil {
		t.Error("Expected error when registering nil plugin")
	}

	// Test registering duplicate plugin name
	plugin1 := &mockPlugin{name: "duplicate", priority: 10}
	err = pm.RegisterPlugin(plugin1)
	if err != nil {
		t.Errorf("Failed to register first plugin: %v", err)
	}

	plugin2 := &mockPlugin{name: "duplicate", priority: 20}
	err = pm.RegisterPlugin(plugin2)
	if err == nil {
		t.Error("Expected error when registering plugin with duplicate name")
	}
}

func TestGetPlugins(t *testing.T) {
	pm := NewPluginManager()

	// Initially should be empty
	plugins := pm.GetPlugins()
	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins initially, got %d", len(plugins))
	}

	// Add some plugins
	plugin1 := &mockPlugin{name: "test1", priority: 10}
	plugin2 := &mockPlugin{name: "test2", priority: 20}

	pm.RegisterPlugin(plugin1)
	pm.RegisterPlugin(plugin2)

	plugins = pm.GetPlugins()
	if len(plugins) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(plugins))
	}

	// Test that returned slice is a copy (modifying it shouldn't affect internal state)
	plugins[0] = nil
	internalPlugins := pm.GetPlugins()
	if internalPlugins[0] == nil {
		t.Error("GetPlugins() should return a copy, not the internal slice")
	}
}

func TestLoadPluginsNonExistentDirectory(t *testing.T) {
	pm := NewPluginManager()

	// Test with non-existent directory (should not error)
	err := pm.LoadPlugins("/non/existent/directory")
	if err != nil {
		t.Errorf("LoadPlugins with non-existent directory should not error: %v", err)
	}
}

func TestLoadPluginsEmptyDirectory(t *testing.T) {
	pm := NewPluginManager()
	tempDir := t.TempDir()

	// Test with empty directory
	err := pm.LoadPlugins(tempDir)
	if err != nil {
		t.Errorf("LoadPlugins with empty directory failed: %v", err)
	}

	plugins := pm.GetPlugins()
	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins after loading empty directory, got %d", len(plugins))
	}
}

func TestLoadPluginsWithNonPluginFiles(t *testing.T) {
	pm := NewPluginManager()
	tempDir := t.TempDir()

	// Create some non-plugin files
	testFiles := []string{"test.txt", "test.go", "test.json"}
	for _, file := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, file), []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Should not error and should not load any plugins
	err := pm.LoadPlugins(tempDir)
	if err != nil {
		t.Errorf("LoadPlugins with non-plugin files failed: %v", err)
	}

	plugins := pm.GetPlugins()
	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins after loading directory with non-plugin files, got %d", len(plugins))
	}
}

func TestLoadPluginsEmptyPath(t *testing.T) {
	pm := NewPluginManager()

	err := pm.LoadPlugins("")
	if err == nil {
		t.Error("Expected error when loading plugins with empty path")
	}
}

func TestExecutePlugins(t *testing.T) {
	pm := NewPluginManager()
	ctx := context.Background()

	baseContext := &types.Context{
		WorkingDirectory: "/test",
		Environment:      make(map[string]string),
		PluginData:       make(map[string]interface{}),
	}

	// Test with no plugins
	data := pm.ExecutePlugins(ctx, baseContext)
	if len(data) != 0 {
		t.Errorf("Expected empty data with no plugins, got %d items", len(data))
	}

	// Add some plugins
	plugin1 := &mockPlugin{
		name:     "test1",
		priority: 10,
		data:     map[string]interface{}{"key1": "value1"},
	}
	plugin2 := &mockPlugin{
		name:     "test2",
		priority: 20,
		data:     map[string]interface{}{"key2": "value2"},
	}

	pm.RegisterPlugin(plugin1)
	pm.RegisterPlugin(plugin2)

	data = pm.ExecutePlugins(ctx, baseContext)
	if len(data) != 2 {
		t.Errorf("Expected 2 plugin data items, got %d", len(data))
	}

	// Check that data from both plugins is present
	if data["test1"] == nil {
		t.Error("Data from test1 plugin not found")
	}
	if data["test2"] == nil {
		t.Error("Data from test2 plugin not found")
	}
}

func TestExecutePluginsWithFailingPlugin(t *testing.T) {
	pm := NewPluginManager()
	ctx := context.Background()

	baseContext := &types.Context{
		WorkingDirectory: "/test",
		Environment:      make(map[string]string),
		PluginData:       make(map[string]interface{}),
	}

	// Add a failing plugin and a successful plugin
	failingPlugin := &mockPlugin{
		name:     "failing",
		priority: 10,
		err:      fmt.Errorf("plugin error"),
	}
	successPlugin := &mockPlugin{
		name:     "success",
		priority: 20,
		data:     map[string]interface{}{"key": "value"},
	}

	pm.RegisterPlugin(failingPlugin)
	pm.RegisterPlugin(successPlugin)

	data := pm.ExecutePlugins(ctx, baseContext)

	// Should only have data from successful plugin
	if len(data) != 1 {
		t.Errorf("Expected 1 plugin data item, got %d", len(data))
	}

	if data["success"] == nil {
		t.Error("Data from successful plugin not found")
	}

	if data["failing"] != nil {
		t.Error("Data from failing plugin should not be present")
	}
}

func TestExecutePluginsWithPanicPlugin(t *testing.T) {
	pm := NewPluginManager()
	ctx := context.Background()

	baseContext := &types.Context{
		WorkingDirectory: "/test",
		Environment:      make(map[string]string),
		PluginData:       make(map[string]interface{}),
	}

	// Add a panicking plugin and a successful plugin
	panicingPlugin := &panicPlugin{name: "panic", priority: 10}
	successPlugin := &mockPlugin{
		name:     "success",
		priority: 20,
		data:     map[string]interface{}{"key": "value"},
	}

	pm.RegisterPlugin(panicingPlugin)
	pm.RegisterPlugin(successPlugin)

	// Should not panic and should continue with other plugins
	data := pm.ExecutePlugins(ctx, baseContext)

	// Should only have data from successful plugin
	if len(data) != 1 {
		t.Errorf("Expected 1 plugin data item, got %d", len(data))
	}

	if data["success"] == nil {
		t.Error("Data from successful plugin not found")
	}

	if data["panic"] != nil {
		t.Error("Data from panicking plugin should not be present")
	}
}

func TestExecutePluginsWithCancellation(t *testing.T) {
	pm := NewPluginManager()
	ctx, cancel := context.WithCancel(context.Background())

	baseContext := &types.Context{
		WorkingDirectory: "/test",
		Environment:      make(map[string]string),
		PluginData:       make(map[string]interface{}),
	}

	// Add a slow plugin
	slowPlugin := &slowPlugin{
		name:     "slow",
		priority: 10,
		delay:    100 * time.Millisecond,
	}

	pm.RegisterPlugin(slowPlugin)

	// Cancel context immediately
	cancel()

	data := pm.ExecutePlugins(ctx, baseContext)

	// Should return empty data due to cancellation
	if len(data) != 0 {
		t.Errorf("Expected empty data due to cancellation, got %d items", len(data))
	}
}

func TestRemovePlugin(t *testing.T) {
	pm := NewPluginManager()

	plugin1 := &mockPlugin{name: "test1", priority: 10}
	plugin2 := &mockPlugin{name: "test2", priority: 20}

	pm.RegisterPlugin(plugin1)
	pm.RegisterPlugin(plugin2)

	// Remove first plugin
	err := pm.RemovePlugin("test1")
	if err != nil {
		t.Errorf("Failed to remove plugin: %v", err)
	}

	plugins := pm.GetPlugins()
	if len(plugins) != 1 {
		t.Errorf("Expected 1 plugin after removal, got %d", len(plugins))
	}

	if plugins[0].Name() != "test2" {
		t.Errorf("Expected remaining plugin to be 'test2', got '%s'", plugins[0].Name())
	}

	// Try to remove non-existent plugin
	err = pm.RemovePlugin("nonexistent")
	if err == nil {
		t.Error("Expected error when removing non-existent plugin")
	}
}

func TestGetPlugin(t *testing.T) {
	pm := NewPluginManager()

	plugin1 := &mockPlugin{name: "test1", priority: 10}
	pm.RegisterPlugin(plugin1)

	// Get existing plugin
	retrieved, err := pm.GetPlugin("test1")
	if err != nil {
		t.Errorf("Failed to get plugin: %v", err)
	}

	if retrieved.Name() != "test1" {
		t.Errorf("Expected plugin name 'test1', got '%s'", retrieved.Name())
	}

	// Try to get non-existent plugin
	_, err = pm.GetPlugin("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent plugin")
	}
}

func TestGetPluginInfo(t *testing.T) {
	pm := NewPluginManager()

	// Initially should be empty
	plugins := pm.GetPlugins()
	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins initially, got %d", len(plugins))
	}

	// Add some plugins
	plugin1 := &mockPlugin{name: "test1", priority: 10}
	plugin2 := &mockPlugin{name: "test2", priority: 20}

	pm.RegisterPlugin(plugin1)
	pm.RegisterPlugin(plugin2)

	plugins = pm.GetPlugins()
	if len(plugins) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(plugins))
	}

	// Check that plugins are sorted by priority (higher first)
	if plugins[0].Name() != "test2" || plugins[0].Priority() != 20 {
		t.Errorf("Expected first plugin to be test2 with priority 20, got %s with priority %d", plugins[0].Name(), plugins[0].Priority())
	}

	if plugins[1].Name() != "test1" || plugins[1].Priority() != 10 {
		t.Errorf("Expected second plugin to be test1 with priority 10, got %s with priority %d", plugins[1].Name(), plugins[1].Priority())
	}
}

func TestClear(t *testing.T) {
	pm := NewPluginManager()

	// Add some plugins
	plugin1 := &mockPlugin{name: "test1", priority: 10}
	plugin2 := &mockPlugin{name: "test2", priority: 20}

	pm.RegisterPlugin(plugin1)
	pm.RegisterPlugin(plugin2)

	plugins := pm.GetPlugins()
	if len(plugins) != 2 {
		t.Errorf("Expected 2 plugins before clear, got %d", len(plugins))
	}

	// Clear all plugins
	pm.Clear()

	plugins = pm.GetPlugins()
	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins after clear, got %d", len(plugins))
	}
}

func TestConcurrentAccess(t *testing.T) {
	pm := NewPluginManager()

	// Test concurrent registration and access
	done := make(chan bool, 2)

	// Goroutine 1: Register plugins
	go func() {
		for i := 0; i < 10; i++ {
			plugin := &mockPlugin{
				name:     fmt.Sprintf("plugin%d", i),
				priority: i,
			}
			pm.RegisterPlugin(plugin)
		}
		done <- true
	}()

	// Goroutine 2: Read plugins
	go func() {
		for i := 0; i < 10; i++ {
			pm.GetPlugins()
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify final state
	plugins := pm.GetPlugins()
	if len(plugins) != 10 {
		t.Errorf("Expected 10 plugins after concurrent access, got %d", len(plugins))
	}
}
