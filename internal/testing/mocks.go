package testing

import (
	"context"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// MockLLMProvider is a mock implementation of interfaces.LLMProvider
type MockLLMProvider struct {
	GenerateCommandFunc func(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error)
	ValidateResultFunc  func(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error)
	GetProviderInfoFunc func() types.ProviderInfo
}

func (m *MockLLMProvider) GenerateCommand(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	if m.GenerateCommandFunc != nil {
		return m.GenerateCommandFunc(ctx, prompt, context)
	}
	return &types.CommandResponse{
		Command:     "echo 'mock command'",
		Explanation: "Mock explanation",
		Confidence:  0.9,
	}, nil
}

func (m *MockLLMProvider) ValidateResult(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	if m.ValidateResultFunc != nil {
		return m.ValidateResultFunc(ctx, command, output, intent)
	}
	return &types.ValidationResponse{
		IsCorrect:   true,
		Explanation: "Mock validation",
	}, nil
}

func (m *MockLLMProvider) GetProviderInfo() types.ProviderInfo {
	if m.GetProviderInfoFunc != nil {
		return m.GetProviderInfoFunc()
	}
	return types.ProviderInfo{
		Name:            "mock",
		RequiresAuth:    false,
		SupportedModels: []string{"mock-model"},
	}
}

// MockContextGatherer is a mock implementation of interfaces.ContextGatherer
type MockContextGatherer struct {
	GatherContextFunc  func(ctx context.Context) (*types.Context, error)
	RegisterPluginFunc func(plugin interfaces.ContextPlugin) error
}

func (m *MockContextGatherer) GatherContext(ctx context.Context) (*types.Context, error) {
	if m.GatherContextFunc != nil {
		return m.GatherContextFunc(ctx)
	}
	return &types.Context{
		WorkingDirectory: "/mock/dir",
		Files:            []types.FileInfo{},
		Environment:      map[string]string{"SHELL": "/bin/bash"},
		PluginData:       map[string]interface{}{},
	}, nil
}

func (m *MockContextGatherer) RegisterPlugin(plugin interfaces.ContextPlugin) error {
	if m.RegisterPluginFunc != nil {
		return m.RegisterPluginFunc(plugin)
	}
	return nil
}

// MockSafetyValidator is a mock implementation of interfaces.SafetyValidator
type MockSafetyValidator struct {
	ValidateCommandFunc            func(command *types.Command) (*types.SafetyResult, error)
	ValidateCommandWithOptionsFunc func(command *types.Command, options *types.ValidationOptions) (*types.SafetyResult, error)
	IsDangerousFunc                func(command string) bool
	GetDangerousPatternsFunc       func() []types.DangerousPattern
}

func (m *MockSafetyValidator) ValidateCommand(command *types.Command) (*types.SafetyResult, error) {
	if m.ValidateCommandFunc != nil {
		return m.ValidateCommandFunc(command)
	}
	return &types.SafetyResult{
		IsSafe:               true,
		DangerLevel:          types.Safe,
		RequiresConfirmation: false,
	}, nil
}

func (m *MockSafetyValidator) ValidateCommandWithOptions(command *types.Command, options *types.ValidationOptions) (*types.SafetyResult, error) {
	if m.ValidateCommandWithOptionsFunc != nil {
		return m.ValidateCommandWithOptionsFunc(command, options)
	}
	return &types.SafetyResult{
		IsSafe:               true,
		DangerLevel:          types.Safe,
		RequiresConfirmation: false,
	}, nil
}

func (m *MockSafetyValidator) GetDangerousPatterns() []types.DangerousPattern {
	if m.GetDangerousPatternsFunc != nil {
		return m.GetDangerousPatternsFunc()
	}
	return []types.DangerousPattern{}
}

func (m *MockSafetyValidator) IsDangerous(command string) bool {
	if m.IsDangerousFunc != nil {
		return m.IsDangerousFunc(command)
	}
	return false
}

// MockExecutor is a mock implementation of interfaces.CommandExecutor
type MockExecutor struct {
	ExecuteFunc func(ctx context.Context, command *types.Command) (*types.ExecutionResult, error)
	DryRunFunc  func(command *types.Command) (*types.DryRunResult, error)
}

func (m *MockExecutor) Execute(ctx context.Context, command *types.Command) (*types.ExecutionResult, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, command)
	}
	return &types.ExecutionResult{
		Command:  command,
		ExitCode: 0,
		Stdout:   "mock output",
		Stderr:   "",
		Duration: 100 * time.Millisecond,
		Success:  true,
	}, nil
}

func (m *MockExecutor) DryRun(command *types.Command) (*types.DryRunResult, error) {
	if m.DryRunFunc != nil {
		return m.DryRunFunc(command)
	}
	return &types.DryRunResult{
		Command:     command,
		Analysis:    "Mock dry run analysis",
		Predictions: []string{"Mock prediction"},
	}, nil
}

// MockResultValidator is a mock implementation of interfaces.ResultValidator
type MockResultValidator struct {
	ValidateResultFunc func(ctx context.Context, result *types.ExecutionResult, originalInput string) (*types.ValidationResult, error)
}

func (m *MockResultValidator) ValidateResult(ctx context.Context, result *types.ExecutionResult, originalInput string) (*types.ValidationResult, error) {
	if m.ValidateResultFunc != nil {
		return m.ValidateResultFunc(ctx, result, originalInput)
	}
	return &types.ValidationResult{
		IsCorrect:   true,
		Explanation: "Mock validation result",
	}, nil
}

// MockCommandManager is a mock implementation of interfaces.CommandManager
type MockCommandManager struct {
	GenerateCommandFunc    func(ctx context.Context, input string) (*types.CommandResult, error)
	ExecuteCommandFunc     func(ctx context.Context, command *types.Command) (*types.ExecutionResult, error)
	GenerateAndExecuteFunc func(ctx context.Context, input string, options *types.ExecutionOptions) (*types.FullResult, error)
	ValidateResultFunc     func(ctx context.Context, result *types.ExecutionResult, originalInput string) (*types.ValidationResult, error)
}

func (m *MockCommandManager) GenerateCommand(ctx context.Context, input string) (*types.CommandResult, error) {
	if m.GenerateCommandFunc != nil {
		return m.GenerateCommandFunc(ctx, input)
	}
	return &types.CommandResult{
		Command: &types.Command{
			ID:        "mock-id",
			Original:  input,
			Generated: "echo 'mock'",
			Validated: true,
		},
		Safety:     &types.SafetyResult{IsSafe: true, DangerLevel: types.Safe},
		Confidence: 0.9,
	}, nil
}

func (m *MockCommandManager) ExecuteCommand(ctx context.Context, command *types.Command) (*types.ExecutionResult, error) {
	if m.ExecuteCommandFunc != nil {
		return m.ExecuteCommandFunc(ctx, command)
	}
	return &types.ExecutionResult{
		Command:  command,
		ExitCode: 0,
		Success:  true,
	}, nil
}

func (m *MockCommandManager) GenerateAndExecute(ctx context.Context, input string, options *types.ExecutionOptions) (*types.FullResult, error) {
	if m.GenerateAndExecuteFunc != nil {
		return m.GenerateAndExecuteFunc(ctx, input, options)
	}
	return &types.FullResult{
		CommandResult: &types.CommandResult{
			Command: &types.Command{
				ID:        "mock-id",
				Original:  input,
				Generated: "echo 'mock'",
				Validated: true,
			},
			Safety:     &types.SafetyResult{IsSafe: true, DangerLevel: types.Safe},
			Confidence: 0.9,
		},
	}, nil
}

func (m *MockCommandManager) ValidateResult(ctx context.Context, result *types.ExecutionResult, originalInput string) (*types.ValidationResult, error) {
	if m.ValidateResultFunc != nil {
		return m.ValidateResultFunc(ctx, result, originalInput)
	}
	return &types.ValidationResult{
		IsCorrect:   true,
		Explanation: "Mock validation",
	}, nil
}

// MockConfigManager is a mock implementation of interfaces.ConfigManager
type MockConfigManager struct {
	LoadFunc                  func() (*types.Config, error)
	SaveFunc                  func(config *types.Config) error
	GetProviderConfigFunc     func(provider string) (*types.ProviderConfig, error)
	SetProviderConfigFunc     func(provider string, config types.ProviderConfig) error
	UpdateUserPreferencesFunc func(prefs types.UserPreferences) error
	ResetFunc                 func() error
}

func (m *MockConfigManager) Load() (*types.Config, error) {
	if m.LoadFunc != nil {
		return m.LoadFunc()
	}
	return &types.Config{
		DefaultProvider: "openai",
		Providers:       make(map[string]types.ProviderConfig),
		UserPreferences: types.UserPreferences{
			DefaultTimeout:  30 * time.Second,
			MaxFileListSize: 100,
			EnablePlugins:   true,
		},
	}, nil
}

func (m *MockConfigManager) Save(config *types.Config) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(config)
	}
	return nil
}

func (m *MockConfigManager) GetProviderConfig(provider string) (*types.ProviderConfig, error) {
	if m.GetProviderConfigFunc != nil {
		return m.GetProviderConfigFunc(provider)
	}
	return &types.ProviderConfig{
		DefaultModel: "mock-model",
		Timeout:      30 * time.Second,
	}, nil
}

func (m *MockConfigManager) SetProviderConfig(provider string, config types.ProviderConfig) error {
	if m.SetProviderConfigFunc != nil {
		return m.SetProviderConfigFunc(provider, config)
	}
	return nil
}

func (m *MockConfigManager) UpdateUserPreferences(prefs types.UserPreferences) error {
	if m.UpdateUserPreferencesFunc != nil {
		return m.UpdateUserPreferencesFunc(prefs)
	}
	return nil
}

func (m *MockConfigManager) Reset() error {
	if m.ResetFunc != nil {
		return m.ResetFunc()
	}
	return nil
}

// MockAuditLogger is a mock implementation of types.AuditLogger
type MockAuditLogger struct {
	LogAuditEventFunc func(entry *types.AuditEntry) error
	GetAuditLogFunc   func(filter *types.AuditFilter) ([]*types.AuditEntry, error)
}

func (m *MockAuditLogger) LogAuditEvent(entry *types.AuditEntry) error {
	if m.LogAuditEventFunc != nil {
		return m.LogAuditEventFunc(entry)
	}
	return nil
}

func (m *MockAuditLogger) GetAuditLog(filter *types.AuditFilter) ([]*types.AuditEntry, error) {
	if m.GetAuditLogFunc != nil {
		return m.GetAuditLogFunc(filter)
	}
	return []*types.AuditEntry{}, nil
}
