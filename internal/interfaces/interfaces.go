package interfaces

import (
	"context"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// LLMProvider defines the interface for Large Language Model providers
type LLMProvider interface {
	GenerateCommand(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error)
	ValidateResult(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error)
	GetProviderInfo() types.ProviderInfo
}

// ContextGatherer defines the interface for gathering environmental context
type ContextGatherer interface {
	GatherContext(ctx context.Context) (*types.Context, error)
	RegisterPlugin(plugin ContextPlugin) error
}

// SafetyValidator defines the interface for validating command safety
type SafetyValidator interface {
	ValidateCommand(cmd *types.Command) (*types.SafetyResult, error)
	IsDangerous(cmd string) bool
	GetDangerousPatterns() []types.DangerousPattern
}

// CommandExecutor defines the interface for executing shell commands
type CommandExecutor interface {
	Execute(ctx context.Context, cmd *types.Command) (*types.ExecutionResult, error)
	DryRun(cmd *types.Command) (*types.DryRunResult, error)
}

// ResultValidator defines the interface for validating command results
type ResultValidator interface {
	ValidateResult(ctx context.Context, result *types.ExecutionResult, intent string) (*types.ValidationResult, error)
}

// CommandManager defines the interface for orchestrating the command generation pipeline
type CommandManager interface {
	GenerateCommand(ctx context.Context, input string) (*types.CommandResult, error)
	ExecuteCommand(ctx context.Context, cmd *types.Command) (*types.ExecutionResult, error)
	ValidateResult(ctx context.Context, result *types.ExecutionResult, intent string) (*types.ValidationResult, error)
}

// ConfigManager defines the interface for configuration management
type ConfigManager interface {
	Load() (*types.Config, error)
	Save(config *types.Config) error
	GetProviderConfig(provider string) (*types.ProviderConfig, error)
	SetupInteractive() error
}

// UpdateManager defines the interface for update management
type UpdateManager interface {
	CheckForUpdates(ctx context.Context) (*types.UpdateInfo, error)
	PerformUpdate(ctx context.Context, updateInfo *types.UpdateInfo) error
	GetCurrentVersion() string
}

// ContextPlugin defines the interface for context plugins
type ContextPlugin interface {
	Name() string
	GatherContext(ctx context.Context, baseContext *types.Context) (map[string]interface{}, error)
	Priority() int
}

// PluginManager defines the interface for managing context plugins
type PluginManager interface {
	RegisterPlugin(plugin ContextPlugin) error
	LoadPlugins(pluginDir string) error
	GetPlugins() []ContextPlugin
	ExecutePlugins(ctx context.Context, baseContext *types.Context) map[string]interface{}
	RemovePlugin(name string) error
	GetPlugin(name string) (ContextPlugin, error)
	Clear()
}

// CLI defines the interface for the command-line interface
type CLI interface {
	Execute() error
	HandleInteractiveSession() error
	DisplayHelp() error
}
