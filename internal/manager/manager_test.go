package manager

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// Mock implementations for testing

type mockContextGatherer struct {
	context *types.Context
	err     error
}

func (m *mockContextGatherer) GatherContext(ctx context.Context) (*types.Context, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.context != nil {
		return m.context, nil
	}
	return &types.Context{
		WorkingDirectory: "/test",
		Environment:      map[string]string{"HOME": "/home/test"},
	}, nil
}

func (m *mockContextGatherer) RegisterPlugin(plugin interfaces.ContextPlugin) error {
	return nil
}

type mockLLMProvider struct {
	response *types.CommandResponse
	err      error
}

func (m *mockLLMProvider) GenerateCommand(ctx context.Context, prompt string, context *types.Context) (*types.CommandResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.response != nil {
		return m.response, nil
	}
	return &types.CommandResponse{
		Command:     "ls -la",
		Explanation: "List files in long format",
		Confidence:  0.9,
	}, nil
}

func (m *mockLLMProvider) ValidateResult(ctx context.Context, command, output, intent string) (*types.ValidationResponse, error) {
	return &types.ValidationResponse{
		IsCorrect:   true,
		Explanation: "Command executed successfully",
	}, nil
}

func (m *mockLLMProvider) GetProviderInfo() types.ProviderInfo {
	return types.ProviderInfo{
		Name:         "mock",
		RequiresAuth: false,
	}
}

type mockSafetyValidator struct {
	result *types.SafetyResult
	err    error
}

func (m *mockSafetyValidator) ValidateCommand(cmd *types.Command) (*types.SafetyResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &types.SafetyResult{
		IsSafe:      true,
		DangerLevel: types.Safe,
	}, nil
}

func (m *mockSafetyValidator) ValidateCommandWithOptions(cmd *types.Command, opts *types.ValidationOptions) (*types.SafetyResult, error) {
	return m.ValidateCommand(cmd)
}

func (m *mockSafetyValidator) IsDangerous(cmd string) bool {
	return false
}

func (m *mockSafetyValidator) GetDangerousPatterns() []types.DangerousPattern {
	return nil
}

type mockExecutor struct {
	result    *types.ExecutionResult
	dryResult *types.DryRunResult
	err       error
}

func (m *mockExecutor) Execute(ctx context.Context, cmd *types.Command) (*types.ExecutionResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &types.ExecutionResult{
		Command:  cmd,
		ExitCode: 0,
		Stdout:   "test output",
		Success:  true,
		Duration: time.Millisecond * 100,
	}, nil
}

func (m *mockExecutor) DryRun(cmd *types.Command) (*types.DryRunResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.dryResult != nil {
		return m.dryResult, nil
	}
	return &types.DryRunResult{
		Command:  cmd,
		Analysis: "This command will list files",
	}, nil
}

type mockResultValidator struct {
	result *types.ValidationResult
	err    error
}

func (m *mockResultValidator) ValidateResult(ctx context.Context, result *types.ExecutionResult, intent string) (*types.ValidationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &types.ValidationResult{
		IsCorrect:   true,
		Explanation: "Command achieved the intended result",
	}, nil
}

func TestManager_GenerateCommand(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		contextErr     error
		llmErr         error
		safetyErr      error
		expectedError  bool
		expectedCmd    string
		expectedSafety bool
	}{
		{
			name:           "successful generation",
			input:          "list files",
			expectedError:  false,
			expectedCmd:    "ls -la",
			expectedSafety: true,
		},
		{
			name:          "context gathering fails",
			input:         "list files",
			contextErr:    errors.New("context error"),
			expectedError: true,
		},
		{
			name:          "LLM provider fails",
			input:         "list files",
			llmErr:        errors.New("llm error"),
			expectedError: true,
		},
		{
			name:          "safety validation fails",
			input:         "list files",
			safetyErr:     errors.New("safety error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			contextGatherer := &mockContextGatherer{err: tt.contextErr}
			llmProvider := &mockLLMProvider{err: tt.llmErr}
			safetyValidator := &mockSafetyValidator{err: tt.safetyErr}
			executor := &mockExecutor{}
			resultValidator := &mockResultValidator{}

			config := &types.Config{
				UserPreferences: types.UserPreferences{
					DefaultTimeout: 30 * time.Second,
				},
			}

			manager := NewManager(
				contextGatherer,
				llmProvider,
				safetyValidator,
				executor,
				resultValidator,
				config,
			)

			ctx := context.Background()
			result, err := manager.GenerateCommand(ctx, tt.input)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected result but got nil")
				return
			}

			if result.Command.Generated != tt.expectedCmd {
				t.Errorf("expected command %q, got %q", tt.expectedCmd, result.Command.Generated)
			}

			if result.Command.Validated != tt.expectedSafety {
				t.Errorf("expected validated %v, got %v", tt.expectedSafety, result.Command.Validated)
			}

			if result.Command.Original != tt.input {
				t.Errorf("expected original %q, got %q", tt.input, result.Command.Original)
			}
		})
	}
}

func TestManager_ExecuteCommand(t *testing.T) {
	tests := []struct {
		name          string
		validated     bool
		executorErr   error
		expectedError bool
	}{
		{
			name:          "successful execution",
			validated:     true,
			expectedError: false,
		},
		{
			name:          "unvalidated command",
			validated:     false,
			expectedError: true,
		},
		{
			name:          "executor fails",
			validated:     true,
			executorErr:   errors.New("execution error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			contextGatherer := &mockContextGatherer{}
			llmProvider := &mockLLMProvider{}
			safetyValidator := &mockSafetyValidator{}
			executor := &mockExecutor{err: tt.executorErr}
			resultValidator := &mockResultValidator{}

			config := &types.Config{}

			manager := NewManager(
				contextGatherer,
				llmProvider,
				safetyValidator,
				executor,
				resultValidator,
				config,
			)

			cmd := &types.Command{
				ID:        "test",
				Generated: "ls -la",
				Validated: tt.validated,
			}

			ctx := context.Background()
			result, err := manager.ExecuteCommand(ctx, cmd)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected result but got nil")
			}
		})
	}
}

func TestManager_GenerateAndExecute(t *testing.T) {
	tests := []struct {
		name                 string
		input                string
		options              *types.ExecutionOptions
		safetyResult         *types.SafetyResult
		expectedDryRun       bool
		expectedConfirmation bool
		expectedExecution    bool
	}{
		{
			name:              "normal execution",
			input:             "list files",
			options:           &types.ExecutionOptions{},
			safetyResult:      &types.SafetyResult{IsSafe: true, DangerLevel: types.Safe},
			expectedExecution: true,
		},
		{
			name:           "dry run mode",
			input:          "list files",
			options:        &types.ExecutionOptions{DryRun: true},
			safetyResult:   &types.SafetyResult{IsSafe: true, DangerLevel: types.Safe},
			expectedDryRun: true,
		},
		{
			name:                 "requires confirmation",
			input:                "delete files",
			options:              &types.ExecutionOptions{},
			safetyResult:         &types.SafetyResult{IsSafe: false, DangerLevel: types.Dangerous, RequiresConfirmation: true},
			expectedConfirmation: true,
		},
		{
			name:              "skip confirmation",
			input:             "delete files",
			options:           &types.ExecutionOptions{SkipConfirmation: true},
			safetyResult:      &types.SafetyResult{IsSafe: false, DangerLevel: types.Dangerous, RequiresConfirmation: true},
			expectedExecution: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			contextGatherer := &mockContextGatherer{}
			llmProvider := &mockLLMProvider{}
			safetyValidator := &mockSafetyValidator{result: tt.safetyResult}
			executor := &mockExecutor{}
			resultValidator := &mockResultValidator{}

			config := &types.Config{
				UserPreferences: types.UserPreferences{
					DefaultTimeout: 30 * time.Second,
				},
			}

			manager := NewManager(
				contextGatherer,
				llmProvider,
				safetyValidator,
				executor,
				resultValidator,
				config,
			)

			ctx := context.Background()
			result, err := manager.GenerateAndExecute(ctx, tt.input, tt.options)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected result but got nil")
				return
			}

			if tt.expectedDryRun && result.DryRunResult == nil {
				t.Errorf("expected dry run result but got nil")
			}

			if tt.expectedConfirmation && !result.RequiresConfirmation {
				t.Errorf("expected confirmation requirement but got false")
			}

			if tt.expectedExecution && result.ExecutionResult == nil {
				t.Errorf("expected execution result but got nil")
			}
		})
	}
}

func TestGenerateCommandID(t *testing.T) {
	id1 := generateCommandID()
	time.Sleep(time.Nanosecond) // Ensure different timestamp
	id2 := generateCommandID()

	if id1 == id2 {
		t.Errorf("expected different IDs, got same: %s", id1)
	}

	if id1 == "" || id2 == "" {
		t.Errorf("expected non-empty IDs, got: %s, %s", id1, id2)
	}
}

func TestManager_GetCommandTimeout(t *testing.T) {
	tests := []struct {
		name            string
		config          *types.Config
		expectedTimeout time.Duration
	}{
		{
			name:            "nil config uses default",
			config:          nil,
			expectedTimeout: 30 * time.Second,
		},
		{
			name: "config with timeout",
			config: &types.Config{
				UserPreferences: types.UserPreferences{
					DefaultTimeout: 60 * time.Second,
				},
			},
			expectedTimeout: 60 * time.Second,
		},
		{
			name: "config with zero timeout uses default",
			config: &types.Config{
				UserPreferences: types.UserPreferences{
					DefaultTimeout: 0,
				},
			},
			expectedTimeout: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{config: tt.config}
			timeout := manager.getCommandTimeout()

			if timeout != tt.expectedTimeout {
				t.Errorf("expected timeout %v, got %v", tt.expectedTimeout, timeout)
			}
		})
	}
}
