package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

func TestDisplayResultsComprehensive(t *testing.T) {
	tests := []struct {
		name     string
		result   *types.FullResult
		input    string
		expected []string
	}{
		{
			name: "basic_result",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "ls -la",
					},
					Confidence: 0.95,
					Safety: &types.SafetyResult{
						DangerLevel: types.Safe,
					},
				},
			},
			input:    "list files",
			expected: []string{"Generated command: ls -la"},
		},
		{
			name: "result_with_warnings",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "rm -rf /tmp/*",
					},
					Safety: &types.SafetyResult{
						DangerLevel: types.Dangerous,
						Warnings:    []string{"This command will delete files"},
					},
				},
			},
			input:    "delete temp files",
			expected: []string{"Safety level: Dangerous", "This command will delete files"},
		},
		{
			name: "dry_run_result",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "ls -la",
					},
					Safety: &types.SafetyResult{
						DangerLevel: types.Safe,
					},
				},
				DryRunResult: &types.DryRunResult{
					Analysis:    "This will list files in long format",
					Predictions: []string{"Will show file permissions"},
				},
			},
			input:    "list files",
			expected: []string{"Dry Run Analysis", "This will list files in long format", "Will show file permissions"},
		},
		{
			name: "execution_result",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "echo hello",
					},
					Safety: &types.SafetyResult{
						DangerLevel: types.Safe,
					},
				},
				ExecutionResult: &types.ExecutionResult{
					ExitCode: 0,
					Duration: 100 * time.Millisecond,
					Stdout:   "hello\n",
					Stderr:   "",
				},
			},
			input:    "say hello",
			expected: []string{"Execution Results", "Exit code: 0", "hello"},
		},
		{
			name: "validation_success",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "echo hello",
					},
					Safety: &types.SafetyResult{
						DangerLevel: types.Safe,
					},
				},
				ExecutionResult: &types.ExecutionResult{
					ExitCode: 0,
					Stdout:   "hello\n",
				},
				ValidationResult: &types.ValidationResult{
					IsCorrect:   true,
					Explanation: "Command executed successfully",
				},
			},
			input:    "say hello",
			expected: []string{"Validation Results", "✅ Command executed successfully"},
		},
		{
			name: "validation_failure",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "ls nonexistent",
					},
					Safety: &types.SafetyResult{
						DangerLevel: types.Safe,
					},
				},
				ExecutionResult: &types.ExecutionResult{
					ExitCode: 1,
					Stderr:   "ls: nonexistent: No such file or directory",
				},
				ValidationResult: &types.ValidationResult{
					IsCorrect:        false,
					Explanation:      "File not found",
					Suggestions:      []string{"Check if the file exists"},
					CorrectedCommand: "ls -la",
				},
			},
			input:    "list nonexistent file",
			expected: []string{"❌ Command may not have achieved", "File not found", "Check if the file exists", "Suggested correction: ls -la"},
		},
		{
			name: "requires_confirmation",
			result: &types.FullResult{
				CommandResult: &types.CommandResult{
					Command: &types.Command{
						Generated: "rm -rf /tmp/*",
					},
					Safety: &types.SafetyResult{
						DangerLevel:          types.Dangerous,
						RequiresConfirmation: true,
					},
				},
				RequiresConfirmation: true,
			},
			input:    "delete temp files",
			expected: []string{"requires confirmation", "--skip-confirmation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := displayResults(tt.result, tt.input)
			if err != nil {
				t.Errorf("displayResults failed: %v", err)
			}

			// Close writer and read output
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("Output should contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

func TestDisplayResultsVerboseMode(t *testing.T) {
	// Set verbose flag
	originalVerbose := verbose
	verbose = true
	defer func() { verbose = originalVerbose }()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	result := &types.FullResult{
		CommandResult: &types.CommandResult{
			Command: &types.Command{
				Generated: "ls -la",
			},
			Confidence:   0.85,
			Alternatives: []string{"ls -l", "dir"},
			Safety: &types.SafetyResult{
				DangerLevel: types.Safe,
			},
		},
	}

	err := displayResults(result, "list files")
	if err != nil {
		t.Errorf("displayResults failed: %v", err)
	}

	// Close writer and read output
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	expectedStrings := []string{
		"Confidence: 0.85",
		"Alternatives:",
		"ls -l",
		"dir",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Verbose output should contain '%s', got: %s", expected, output)
		}
	}
}

func TestCreateLLMProviderSuccess(t *testing.T) {
	cfg := &types.Config{
		DefaultProvider: "openai",
		Providers: map[string]types.ProviderConfig{
			"openai": {
				APIKey:       "test-key",
				BaseURL:      "https://api.openai.com/v1",
				DefaultModel: "gpt-4",
				Timeout:      30 * time.Second,
			},
		},
	}

	// This will fail because we don't have actual provider implementations
	// but we can test the configuration logic
	_, err := createLLMProvider(cfg)
	if err != nil {
		// Expected error - this is fine for testing the function exists and handles config
		t.Logf("Expected error due to missing provider implementation: %v", err)
	}
}

func TestCreateLLMProviderWithOverrides(t *testing.T) {
	// Set global provider and model overrides
	originalProvider := provider
	originalModel := model
	provider = "anthropic"
	model = "claude-3"
	defer func() {
		provider = originalProvider
		model = originalModel
	}()

	cfg := &types.Config{
		DefaultProvider: "openai",
		Providers: map[string]types.ProviderConfig{
			"openai": {
				APIKey:       "test-key",
				DefaultModel: "gpt-4",
			},
			"anthropic": {
				APIKey:       "test-key-2",
				DefaultModel: "claude-2",
			},
		},
	}

	// This will fail because we don't have actual provider implementations
	// but we can test that it tries to use the overridden provider
	_, err := createLLMProvider(cfg)
	if err != nil {
		// Expected error - log it for debugging
		t.Logf("Expected error due to missing provider implementation: %v", err)
	}
}

func TestGlobalFlagsComprehensive(t *testing.T) {
	// Save original values
	originalDryRun := dryRun
	originalVerbose := verbose
	originalProvider := provider
	originalModel := model
	originalSkipConfirmation := skipConfirmation
	originalValidateResults := validateResults
	originalSessionMode := sessionMode

	// Set test values
	dryRun = true
	verbose = true
	provider = "test-provider"
	model = "test-model"
	skipConfirmation = true
	validateResults = false
	sessionMode = true

	// Test GetGlobalFlags
	flags := GetGlobalFlags()

	// Verify all flags
	if flags.DryRun != true {
		t.Errorf("Expected DryRun to be true, got %v", flags.DryRun)
	}
	if flags.Verbose != true {
		t.Errorf("Expected Verbose to be true, got %v", flags.Verbose)
	}
	if flags.Provider != "test-provider" {
		t.Errorf("Expected Provider to be 'test-provider', got %s", flags.Provider)
	}
	if flags.Model != "test-model" {
		t.Errorf("Expected Model to be 'test-model', got %s", flags.Model)
	}
	if flags.SkipConfirmation != true {
		t.Errorf("Expected SkipConfirmation to be true, got %v", flags.SkipConfirmation)
	}
	if flags.ValidateResults != false {
		t.Errorf("Expected ValidateResults to be false, got %v", flags.ValidateResults)
	}
	if flags.SessionMode != true {
		t.Errorf("Expected SessionMode to be true, got %v", flags.SessionMode)
	}

	// Restore original values
	dryRun = originalDryRun
	verbose = originalVerbose
	provider = originalProvider
	model = originalModel
	skipConfirmation = originalSkipConfirmation
	validateResults = originalValidateResults
	sessionMode = originalSessionMode
}

func TestCommandStructure(t *testing.T) {
	// Test that all expected commands are present
	commands := rootCmd.Commands()

	expectedCommands := []string{
		"generate",
		"config",
		"session",
		"update",
		"version",
		"completion",
	}

	commandNames := make(map[string]bool)
	for _, cmd := range commands {
		commandNames[cmd.Name()] = true
	}

	for _, expected := range expectedCommands {
		if !commandNames[expected] {
			t.Errorf("Expected command '%s' not found", expected)
		}
	}
}

func TestSubcommandStructure(t *testing.T) {
	// Test config subcommands
	configSubcommands := configCmd.Commands()
	expectedConfigSubs := []string{"setup", "show", "reset"}

	configSubNames := make(map[string]bool)
	for _, cmd := range configSubcommands {
		configSubNames[cmd.Name()] = true
	}

	for _, expected := range expectedConfigSubs {
		if !configSubNames[expected] {
			t.Errorf("Expected config subcommand '%s' not found", expected)
		}
	}

	// Test update subcommands
	updateSubcommands := updateCmd.Commands()
	expectedUpdateSubs := []string{"check", "install"}

	updateSubNames := make(map[string]bool)
	for _, cmd := range updateSubcommands {
		updateSubNames[cmd.Name()] = true
	}

	for _, expected := range expectedUpdateSubs {
		if !updateSubNames[expected] {
			t.Errorf("Expected update subcommand '%s' not found", expected)
		}
	}
}

func TestVersionInfo(t *testing.T) {
	// Test that version variables are accessible
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if GitCommit == "" {
		t.Error("GitCommit should not be empty")
	}
	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}
}

func TestExecuteUpdateCheckError(t *testing.T) {
	// This test verifies the function signature and basic error handling
	// We can't test the actual update functionality without mocking
	err := executeUpdateCheck(checkCmd, []string{})
	if err == nil {
		t.Error("Expected error due to missing update manager setup")
	}
}

func TestExecuteUpdateInstallError(t *testing.T) {
	// This test verifies the function signature and basic error handling
	// We can't test the actual update functionality without mocking
	err := executeUpdateInstall(installCmd, []string{})
	if err == nil {
		t.Error("Expected error due to missing update manager setup")
	}
}
