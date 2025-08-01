package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/config"
	contextpkg "github.com/kanishka-sahoo/nl-to-shell/internal/context"
	"github.com/kanishka-sahoo/nl-to-shell/internal/executor"
	"github.com/kanishka-sahoo/nl-to-shell/internal/manager"
	"github.com/kanishka-sahoo/nl-to-shell/internal/safety"
	mocks "github.com/kanishka-sahoo/nl-to-shell/internal/testing"
	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

func TestFullPipelineIntegration(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "nl-to-shell-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test configuration
	cfg := &types.Config{
		DefaultProvider: "mock",
		Providers: map[string]types.ProviderConfig{
			"mock": {
				APIKey:       "test-key",
				DefaultModel: "test-model",
				Timeout:      30 * time.Second,
			},
		},
		UserPreferences: types.UserPreferences{
			DefaultTimeout:  30 * time.Second,
			MaxFileListSize: 100,
			EnablePlugins:   true,
		},
	}

	// Create components with mocks
	contextGatherer := &mocks.MockContextGatherer{
		GatherContextFunc: func(ctx context.Context) (*types.Context, error) {
			return &types.Context{
				WorkingDirectory: tempDir,
				Files:            []types.FileInfo{},
				Environment:      map[string]string{"SHELL": "/bin/bash"},
				PluginData:       map[string]interface{}{},
			}, nil
		},
	}

	safetyValidator := &mocks.MockSafetyValidator{
		ValidateCommandFunc: func(command *types.Command) (*types.SafetyResult, error) {
			return &types.SafetyResult{
				IsSafe:               true,
				DangerLevel:          types.Safe,
				RequiresConfirmation: false,
			}, nil
		},
	}

	commandExecutor := &mocks.MockExecutor{
		ExecuteFunc: func(ctx context.Context, command *types.Command) (*types.ExecutionResult, error) {
			return &types.ExecutionResult{
				Command:  command,
				ExitCode: 0,
				Stdout:   "test output",
				Stderr:   "",
				Duration: 100 * time.Millisecond,
				Success:  true,
			}, nil
		},
	}

	llmProvider := &mocks.MockLLMProvider{
		GenerateCommandFunc: func(ctx context.Context, input string, context *types.Context) (*types.CommandResponse, error) {
			return &types.CommandResponse{
				Command:     "echo 'integration test'",
				Explanation: "Test command for integration",
				Confidence:  0.9,
			}, nil
		},
	}

	resultValidator := &mocks.MockResultValidator{
		ValidateResultFunc: func(ctx context.Context, result *types.ExecutionResult, originalInput string) (*types.ValidationResult, error) {
			return &types.ValidationResult{
				IsCorrect:   true,
				Explanation: "Integration test validation",
			}, nil
		},
	}

	// Create command manager
	commandManager := manager.NewManager(
		contextGatherer,
		llmProvider,
		safetyValidator,
		commandExecutor,
		resultValidator,
		cfg,
	)

	// Test full pipeline
	ctx := context.Background()
	options := &types.ExecutionOptions{
		DryRun:           false,
		SkipConfirmation: true,
		ValidateResults:  true,
		Timeout:          30 * time.Second,
	}

	result, err := commandManager.GenerateAndExecute(ctx, "test command", options)
	if err != nil {
		t.Fatalf("Full pipeline failed: %v", err)
	}

	// Verify results
	if result.CommandResult == nil {
		t.Error("CommandResult should not be nil")
	}
	if result.ExecutionResult == nil {
		t.Error("ExecutionResult should not be nil")
	}
	if result.ValidationResult == nil {
		t.Error("ValidationResult should not be nil")
	}

	if result.CommandResult.Command.Generated != "echo 'integration test'" {
		t.Errorf("Expected generated command 'echo 'integration test'', got %s", result.CommandResult.Command.Generated)
	}
	if result.ExecutionResult.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExecutionResult.ExitCode)
	}
	if !result.ValidationResult.IsCorrect {
		t.Error("Expected validation to be correct")
	}
}

func TestConfigurationIntegration(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "nl-to-shell-config-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create config manager with custom path
	configManager := config.NewManager()

	// Test configuration save and load cycle
	testConfig := &types.Config{
		DefaultProvider: "openai",
		Providers: map[string]types.ProviderConfig{
			"openai": {
				APIKey:       "test-api-key",
				BaseURL:      "https://api.openai.com/v1",
				DefaultModel: "gpt-4",
				Timeout:      45 * time.Second,
			},
		},
		UserPreferences: types.UserPreferences{
			SkipConfirmation: true,
			VerboseOutput:    false,
			DefaultTimeout:   60 * time.Second,
			MaxFileListSize:  200,
			EnablePlugins:    true,
			AutoUpdate:       false,
		},
		UpdateSettings: types.UpdateSettings{
			AutoCheck:          false,
			CheckInterval:      48 * time.Hour,
			AllowPrerelease:    true,
			BackupBeforeUpdate: false,
		},
	}

	// Save configuration
	err = configManager.Save(testConfig)
	if err != nil {
		t.Fatalf("Failed to save configuration: %v", err)
	}

	// Load configuration
	loadedConfig, err := configManager.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Verify configuration integrity
	if loadedConfig.DefaultProvider != testConfig.DefaultProvider {
		t.Errorf("DefaultProvider mismatch: expected %s, got %s", testConfig.DefaultProvider, loadedConfig.DefaultProvider)
	}

	openaiConfig, exists := loadedConfig.Providers["openai"]
	if !exists {
		t.Fatal("OpenAI provider config not found")
	}
	if openaiConfig.APIKey != "test-api-key" {
		t.Errorf("API key mismatch: expected test-api-key, got %s", openaiConfig.APIKey)
	}

	if loadedConfig.UserPreferences.DefaultTimeout != 60*time.Second {
		t.Errorf("DefaultTimeout mismatch: expected 60s, got %v", loadedConfig.UserPreferences.DefaultTimeout)
	}
}

func TestContextGatheringIntegration(t *testing.T) {
	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "nl-to-shell-context-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := []string{"test1.txt", "test2.go", "README.md"}
	for _, filename := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create context gatherer
	gatherer := contextpkg.NewGatherer()

	// Gather context
	ctx := context.Background()
	contextInfo, err := gatherer.GatherContext(ctx)
	if err != nil {
		t.Fatalf("Failed to gather context: %v", err)
	}

	// Verify context (resolve symlinks for comparison on macOS)
	expectedDir, _ := filepath.EvalSymlinks(tempDir)
	actualDir, _ := filepath.EvalSymlinks(contextInfo.WorkingDirectory)
	if actualDir != expectedDir {
		t.Errorf("Working directory mismatch: expected %s, got %s", expectedDir, actualDir)
	}

	if len(contextInfo.Files) == 0 {
		t.Error("Expected files to be gathered")
	}

	// Check that test files are included
	fileNames := make(map[string]bool)
	for _, file := range contextInfo.Files {
		fileNames[file.Name] = true
	}

	for _, expectedFile := range testFiles {
		if !fileNames[expectedFile] {
			t.Errorf("Expected file %s not found in context", expectedFile)
		}
	}

	if contextInfo.Environment == nil {
		t.Error("Environment should not be nil")
	}
}

func TestSafetyValidationIntegration(t *testing.T) {
	validator := safety.NewValidator()

	testCases := []struct {
		name            string
		command         string
		expectedSafe    bool
		expectedLevel   types.DangerLevel
		expectedConfirm bool
	}{
		{
			name:            "safe_command",
			command:         "ls -la",
			expectedSafe:    true,
			expectedLevel:   types.Safe,
			expectedConfirm: false,
		},
		{
			name:            "dangerous_command",
			command:         "rm -rf /tmp/*",
			expectedSafe:    false,
			expectedLevel:   types.Dangerous,
			expectedConfirm: true,
		},
		{
			name:            "critical_command",
			command:         "rm -rf /",
			expectedSafe:    false,
			expectedLevel:   types.Critical,
			expectedConfirm: true,
		},
		{
			name:            "warning_command",
			command:         "chmod 777 file.txt",
			expectedSafe:    false,
			expectedLevel:   types.Dangerous,
			expectedConfirm: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			command := &types.Command{
				ID:        "test-id",
				Original:  tc.command,
				Generated: tc.command,
				Validated: false,
			}

			result, err := validator.ValidateCommand(command)
			if err != nil {
				t.Fatalf("Validation failed: %v", err)
			}

			if result.IsSafe != tc.expectedSafe {
				t.Errorf("Expected IsSafe %v, got %v", tc.expectedSafe, result.IsSafe)
			}
			if result.DangerLevel != tc.expectedLevel {
				t.Errorf("Expected DangerLevel %v, got %v", tc.expectedLevel, result.DangerLevel)
			}
			if result.RequiresConfirmation != tc.expectedConfirm {
				t.Errorf("Expected RequiresConfirmation %v, got %v", tc.expectedConfirm, result.RequiresConfirmation)
			}
		})
	}
}

func TestExecutorIntegration(t *testing.T) {
	executor := executor.NewExecutor()
	ctx := context.Background()

	testCases := []struct {
		name           string
		command        string
		expectedOutput string
		shouldSucceed  bool
	}{
		{
			name:           "echo_command",
			command:        "echo 'hello world'",
			expectedOutput: "hello world",
			shouldSucceed:  true,
		},
		{
			name:           "pwd_command",
			command:        "pwd",
			expectedOutput: "",
			shouldSucceed:  true,
		},
		{
			name:           "invalid_command",
			command:        "nonexistentcommand12345",
			expectedOutput: "",
			shouldSucceed:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			command := &types.Command{
				ID:        "test-id",
				Generated: tc.command,
				Validated: true,
			}

			result, err := executor.Execute(ctx, command)
			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected command to succeed, got error: %v", err)
				}
				if result.ExitCode != 0 {
					t.Errorf("Expected exit code 0, got %d", result.ExitCode)
				}
				if tc.expectedOutput != "" && result.Stdout != tc.expectedOutput+"\n" {
					t.Errorf("Expected output '%s', got '%s'", tc.expectedOutput, result.Stdout)
				}
			} else {
				if err == nil && result.ExitCode == 0 {
					t.Error("Expected command to fail")
				}
			}
		})
	}
}

func TestDryRunIntegration(t *testing.T) {
	executor := executor.NewExecutor()

	command := &types.Command{
		ID:        "test-id",
		Generated: "ls -la /tmp",
		Validated: false,
	}

	result, err := executor.DryRun(command)
	if err != nil {
		t.Fatalf("Dry run failed: %v", err)
	}

	if result.Command != command {
		t.Error("Dry run result should contain the original command")
	}
	if result.Analysis == "" {
		t.Error("Dry run should provide analysis")
	}
	if len(result.Predictions) == 0 {
		t.Error("Dry run should provide predictions")
	}
}

func TestCrossComponentIntegration(t *testing.T) {
	// Test that components work together correctly
	ctx := context.Background()

	// Create real components (not mocks) to test actual integration
	contextGatherer := contextpkg.NewGatherer()
	safetyValidator := safety.NewValidator()
	commandExecutor := executor.NewExecutor()

	// Test context gathering
	contextInfo, err := contextGatherer.GatherContext(ctx)
	if err != nil {
		t.Fatalf("Context gathering failed: %v", err)
	}

	// Test safety validation with a safe command
	safeCommand := &types.Command{
		ID:        "test-safe",
		Generated: "echo 'test'",
		Context:   contextInfo,
	}

	safetyResult, err := safetyValidator.ValidateCommand(safeCommand)
	if err != nil {
		t.Fatalf("Safety validation failed: %v", err)
	}

	if !safetyResult.IsSafe {
		t.Error("Echo command should be safe")
	}

	// Mark as validated and execute
	safeCommand.Validated = true
	execResult, err := commandExecutor.Execute(ctx, safeCommand)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	if execResult.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", execResult.ExitCode)
	}

	// Test with dangerous command
	dangerousCommand := &types.Command{
		ID:        "test-dangerous",
		Generated: "rm -rf /tmp/nonexistent",
		Context:   contextInfo,
	}

	dangerousResult, err := safetyValidator.ValidateCommand(dangerousCommand)
	if err != nil {
		t.Fatalf("Safety validation failed: %v", err)
	}

	if dangerousResult.IsSafe {
		t.Error("rm -rf command should not be safe")
	}
	if dangerousResult.DangerLevel < types.Dangerous {
		t.Errorf("Expected danger level >= Dangerous, got %v", dangerousResult.DangerLevel)
	}
}

func TestErrorHandlingIntegration(t *testing.T) {
	// Test error propagation through the system
	ctx := context.Background()

	// Create components
	contextGatherer := &mocks.MockContextGatherer{
		GatherContextFunc: func(ctx context.Context) (*types.Context, error) {
			return nil, &types.NLShellError{
				Type:     types.ErrTypeContext,
				Severity: types.SeverityError,
				Message:  "Mock context error",
			}
		},
	}

	llmProvider := &mocks.MockLLMProvider{}
	safetyValidator := &mocks.MockSafetyValidator{}
	commandExecutor := &mocks.MockExecutor{}
	resultValidator := &mocks.MockResultValidator{}

	cfg := &types.Config{
		DefaultProvider: "mock",
		UserPreferences: types.UserPreferences{
			DefaultTimeout: 30 * time.Second,
		},
	}

	commandManager := manager.NewManager(
		contextGatherer,
		llmProvider,
		safetyValidator,
		commandExecutor,
		resultValidator,
		cfg,
	)

	// Test that context error propagates
	_, err := commandManager.GenerateCommand(ctx, "test command")
	if err == nil {
		t.Error("Expected error from context gathering")
	}

	// Verify error type
	var nlErr *types.NLShellError
	if !errors.As(err, &nlErr) {
		t.Error("Expected NLShellError type")
	} else {
		if nlErr.Type != types.ErrTypeValidation {
			t.Errorf("Expected validation error type, got %v", nlErr.Type)
		}
	}
}

func TestTimeoutIntegration(t *testing.T) {
	// Test timeout handling across components
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Create executor with a shorter default timeout to ensure our context timeout takes precedence
	executor := executor.NewExecutorWithTimeout(50 * time.Millisecond)

	// Use a command that will definitely take longer than 100ms
	command := &types.Command{
		ID:        "test-timeout",
		Generated: "sleep 1", // This should timeout after 50ms (executor timeout)
		Validated: true,
	}

	start := time.Now()
	result, err := executor.Execute(ctx, command)
	duration := time.Since(start)

	// The Execute method should succeed (err == nil) but the result should contain the timeout error
	if err != nil {
		t.Errorf("Execute method should not return error, got: %v", err)
	}

	// The result should indicate failure and contain a timeout error
	if result.Success {
		t.Error("Expected command execution to fail due to timeout")
	}

	if result.Error == nil {
		t.Error("Expected timeout error in result.Error")
	} else {
		// Check if it's a timeout error
		var nlErr *types.NLShellError
		if errors.As(result.Error, &nlErr) {
			if nlErr.Type != types.ErrTypeTimeout {
				t.Errorf("Expected timeout error type, got %v", nlErr.Type)
			}
		} else {
			t.Errorf("Expected NLShellError, got %T", result.Error)
		}
	}

	// The execution should have taken roughly the timeout duration, not the full sleep duration
	if duration > 200*time.Millisecond {
		t.Errorf("Command took too long (%v), expected to be interrupted by timeout", duration)
	}

	// The exit code should indicate failure
	if result.ExitCode == 0 {
		t.Error("Expected non-zero exit code for timed out command")
	}
}
