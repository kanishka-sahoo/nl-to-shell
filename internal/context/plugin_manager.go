package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sort"
	"sync"

	"github.com/kanishka-sahoo/nl-to-shell/internal/interfaces"
	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

// PluginManager manages context plugins with priority system and error isolation
type PluginManager struct {
	plugins []interfaces.ContextPlugin
	mutex   sync.RWMutex
}

// NewPluginManager creates a new plugin manager
func NewPluginManager() interfaces.PluginManager {
	return &PluginManager{
		plugins: make([]interfaces.ContextPlugin, 0),
	}
}

// RegisterPlugin registers a context plugin with priority-based ordering
func (pm *PluginManager) RegisterPlugin(contextPlugin interfaces.ContextPlugin) error {
	if contextPlugin == nil {
		return &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "plugin cannot be nil",
		}
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Check if plugin with same name already exists
	for _, existingPlugin := range pm.plugins {
		if existingPlugin.Name() == contextPlugin.Name() {
			return &types.NLShellError{
				Type:    types.ErrTypeValidation,
				Message: fmt.Sprintf("plugin with name '%s' already registered", contextPlugin.Name()),
			}
		}
	}

	pm.plugins = append(pm.plugins, contextPlugin)

	// Sort plugins by priority (higher priority first)
	sort.Slice(pm.plugins, func(i, j int) bool {
		return pm.plugins[i].Priority() > pm.plugins[j].Priority()
	})

	return nil
}

// LoadPlugins loads plugins from a directory (for dynamic plugin loading)
func (pm *PluginManager) LoadPlugins(pluginDir string) error {
	if pluginDir == "" {
		return &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "plugin directory cannot be empty",
		}
	}

	// Check if plugin directory exists
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		// Directory doesn't exist, but this is not an error - just no plugins to load
		return nil
	}

	// Walk through the plugin directory
	err := filepath.Walk(pluginDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip files we can't access
			return nil
		}

		// Only process .so files (shared libraries)
		if filepath.Ext(path) != ".so" {
			return nil
		}

		// Try to load the plugin
		if err := pm.loadPluginFromFile(path); err != nil {
			// Log error but continue loading other plugins
			// In a real implementation, you might want to use a proper logger here
			fmt.Printf("Warning: failed to load plugin %s: %v\n", path, err)
		}

		return nil
	})

	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: "failed to walk plugin directory",
			Cause:   err,
		}
	}

	return nil
}

// GetPlugins returns a copy of all registered plugins
func (pm *PluginManager) GetPlugins() []interfaces.ContextPlugin {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	// Return a copy to prevent external modification
	plugins := make([]interfaces.ContextPlugin, len(pm.plugins))
	copy(plugins, pm.plugins)
	return plugins
}

// loadPluginFromFile loads a plugin from a shared library file
func (pm *PluginManager) loadPluginFromFile(path string) error {
	// Open the plugin
	p, err := plugin.Open(path)
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: fmt.Sprintf("failed to open plugin %s", path),
			Cause:   err,
		}
	}

	// Look for the NewPlugin function
	newPluginSymbol, err := p.Lookup("NewPlugin")
	if err != nil {
		return &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: fmt.Sprintf("plugin %s does not export NewPlugin function", path),
			Cause:   err,
		}
	}

	// Cast to the expected function type
	newPluginFunc, ok := newPluginSymbol.(func() interfaces.ContextPlugin)
	if !ok {
		return &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: fmt.Sprintf("plugin %s NewPlugin function has wrong signature", path),
		}
	}

	// Create the plugin instance
	contextPlugin := newPluginFunc()
	if contextPlugin == nil {
		return &types.NLShellError{
			Type:    types.ErrTypeValidation,
			Message: fmt.Sprintf("plugin %s NewPlugin function returned nil", path),
		}
	}

	// Register the plugin
	return pm.RegisterPlugin(contextPlugin)
}

// ExecutePlugins executes all registered plugins with error isolation
func (pm *PluginManager) ExecutePlugins(ctx context.Context, baseContext *types.Context) map[string]interface{} {
	pm.mutex.RLock()
	plugins := make([]interfaces.ContextPlugin, len(pm.plugins))
	copy(plugins, pm.plugins)
	pm.mutex.RUnlock()

	pluginData := make(map[string]interface{})

	for _, contextPlugin := range plugins {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return pluginData
		default:
		}

		// Execute plugin with error isolation
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Plugin panicked, log and continue with other plugins
					fmt.Printf("Warning: plugin %s panicked: %v\n", contextPlugin.Name(), r)
				}
			}()

			data, err := contextPlugin.GatherContext(ctx, baseContext)
			if err != nil {
				// Plugin failed, log and continue with other plugins
				fmt.Printf("Warning: plugin %s failed: %v\n", contextPlugin.Name(), err)
				return
			}

			if data != nil {
				pluginData[contextPlugin.Name()] = data
			}
		}()
	}

	return pluginData
}

// RemovePlugin removes a plugin by name
func (pm *PluginManager) RemovePlugin(name string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	for i, contextPlugin := range pm.plugins {
		if contextPlugin.Name() == name {
			// Remove plugin from slice
			pm.plugins = append(pm.plugins[:i], pm.plugins[i+1:]...)
			return nil
		}
	}

	return &types.NLShellError{
		Type:    types.ErrTypeValidation,
		Message: fmt.Sprintf("plugin with name '%s' not found", name),
	}
}

// GetPlugin returns a plugin by name
func (pm *PluginManager) GetPlugin(name string) (interfaces.ContextPlugin, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	for _, contextPlugin := range pm.plugins {
		if contextPlugin.Name() == name {
			return contextPlugin, nil
		}
	}

	return nil, &types.NLShellError{
		Type:    types.ErrTypeValidation,
		Message: fmt.Sprintf("plugin with name '%s' not found", name),
	}
}

// PluginInfo contains information about a registered plugin
type PluginInfo struct {
	Name     string
	Priority int
}

// GetPluginInfo returns information about all registered plugins
func (pm *PluginManager) GetPluginInfo() []PluginInfo {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	info := make([]PluginInfo, len(pm.plugins))
	for i, contextPlugin := range pm.plugins {
		info[i] = PluginInfo{
			Name:     contextPlugin.Name(),
			Priority: contextPlugin.Priority(),
		}
	}

	return info
}

// Clear removes all registered plugins
func (pm *PluginManager) Clear() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.plugins = make([]interfaces.ContextPlugin, 0)
}
