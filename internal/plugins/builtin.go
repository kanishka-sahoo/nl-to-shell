package plugins

import (
	"github.com/kanishka-sahoo/nl-to-shell/internal/interfaces"
)

// RegisterBuiltinPlugins registers all built-in context plugins with the plugin manager
func RegisterBuiltinPlugins(pluginManager interfaces.PluginManager) error {
	// Register environment variable plugin
	envPlugin := NewEnvPlugin()
	if err := pluginManager.RegisterPlugin(envPlugin); err != nil {
		return err
	}

	// Register development tools plugin
	devToolsPlugin := NewDevToolsPlugin()
	if err := pluginManager.RegisterPlugin(devToolsPlugin); err != nil {
		return err
	}

	// Register project type identification plugin
	projectPlugin := NewProjectPlugin()
	if err := pluginManager.RegisterPlugin(projectPlugin); err != nil {
		return err
	}

	return nil
}

// GetBuiltinPlugins returns a list of all built-in plugins
func GetBuiltinPlugins() []interfaces.ContextPlugin {
	return []interfaces.ContextPlugin{
		NewEnvPlugin(),
		NewDevToolsPlugin(),
		NewProjectPlugin(),
	}
}
