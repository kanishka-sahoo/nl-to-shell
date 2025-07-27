package plugins

import (
	"testing"

	"github.com/nl-to-shell/nl-to-shell/internal/context"
)

func TestRegisterBuiltinPlugins(t *testing.T) {
	// Create a new plugin manager
	pluginManager := context.NewPluginManager()

	// Register built-in plugins
	err := RegisterBuiltinPlugins(pluginManager)
	if err != nil {
		t.Fatalf("Failed to register built-in plugins: %v", err)
	}

	// Check that all plugins are registered
	plugins := pluginManager.GetPlugins()
	if len(plugins) != 3 {
		t.Errorf("Expected 3 plugins, got %d", len(plugins))
	}

	// Check that specific plugins are present
	expectedPlugins := map[string]bool{
		"environment": false,
		"devtools":    false,
		"project":     false,
	}

	for _, plugin := range plugins {
		if _, exists := expectedPlugins[plugin.Name()]; exists {
			expectedPlugins[plugin.Name()] = true
		}
	}

	for pluginName, found := range expectedPlugins {
		if !found {
			t.Errorf("Expected plugin '%s' to be registered", pluginName)
		}
	}
}

func TestRegisterBuiltinPlugins_DuplicateRegistration(t *testing.T) {
	// Create a new plugin manager
	pluginManager := context.NewPluginManager()

	// Register built-in plugins twice
	err := RegisterBuiltinPlugins(pluginManager)
	if err != nil {
		t.Fatalf("Failed to register built-in plugins first time: %v", err)
	}

	err = RegisterBuiltinPlugins(pluginManager)
	if err == nil {
		t.Error("Expected error when registering plugins twice")
	}
}

func TestGetBuiltinPlugins(t *testing.T) {
	plugins := GetBuiltinPlugins()

	if len(plugins) != 3 {
		t.Errorf("Expected 3 built-in plugins, got %d", len(plugins))
	}

	// Check that all plugins have valid names and priorities
	expectedPlugins := map[string]int{
		"environment": 100,
		"devtools":    90,
		"project":     80,
	}

	for _, plugin := range plugins {
		expectedPriority, exists := expectedPlugins[plugin.Name()]
		if !exists {
			t.Errorf("Unexpected plugin '%s'", plugin.Name())
			continue
		}

		if plugin.Priority() != expectedPriority {
			t.Errorf("Expected plugin '%s' to have priority %d, got %d",
				plugin.Name(), expectedPriority, plugin.Priority())
		}

		if plugin.Name() == "" {
			t.Errorf("Plugin name should not be empty")
		}
	}
}
